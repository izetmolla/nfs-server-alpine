package nfs

import (
	"fmt"
	"path"
	"strings"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
)

type Choice struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Help    string `json:"help"`
	Warning string `json:"warning,omitempty"`
}

type FieldCatalog struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Help     string   `json:"help"`
	Summary  string   `json:"summary"`
	Examples []string `json:"examples,omitempty"`
	Cases    []string `json:"cases,omitempty"`
	Type     string   `json:"type"` // enum | bool | number
	Choices  []Choice `json:"choices,omitempty"`
	Default  any      `json:"default"`
	Advanced bool     `json:"advanced"`
	Risk     string   `json:"risk,omitempty"` // low | medium | high
}

type OptionCatalog struct {
	Fields  []FieldCatalog `json:"fields"`
	Default models.Export  `json:"default"`
	Notes   []string       `json:"notes"`
}

func Catalog() OptionCatalog {
	def := models.DefaultExport()
	return OptionCatalog{
		Default: def,
		Notes: []string{
			"This server runs NFSv4 only (nfsd --no-nfs-version 2/3). hide/nohide mainly affect NFSv3 clients.",
			"Exactly one enabled export should use fsid=0 as the NFSv4 pseudo-root.",
			"Additional exports should be directories under the fsid=0 path.",
			"On Apply, missing export directories are created automatically (mkdir -p).",
		},
		Fields: []FieldCatalog{
			{
				Key: "path", Label: "Export path", Type: "path", Default: def.Path, Risk: "medium",
				Summary: "Absolute directory on the server that NFS clients will mount.",
				Help: "Must be an absolute path under the shared volume (typically /mnt). On Save & Apply the manager runs mkdir -p for this path before writing /etc/exports. For NFSv4, the fsid=0 export is the pseudo-root; other shares should be child directories of that root.",
				Examples: []string{"/mnt", "/mnt/media", "/mnt/backups/nas"},
				Cases: []string{
					"Path does not exist yet → Apply creates it with mode 0755.",
					"Path outside the shared volume → export may work inside the container but data will not persist on the host bind mount.",
					"Second export not under fsid=0 path → validation fails (NFSv4 tree rule).",
				},
			},
			{
				Key: "clients", Label: "Clients", Type: "clients", Default: def.Clients, Risk: "high",
				Summary: "Which hosts are allowed to mount this export.",
				Help: "Space-separated list of *, IPv4/IPv6 addresses, CIDR networks, or hostnames. * means every client that can reach the server. Prefer narrowing to your LAN CIDR (e.g. 192.168.200.0/24) in production.",
				Examples: []string{"*", "192.168.200.0/24", "10.0.0.15 10.0.0.16", "laptop.local"},
				Cases: []string{
					"* + rw + no_root_squash → any host can write as root on the share (very dangerous on untrusted networks).",
					"Wrong CIDR → clients get permission denied / mount fails even though NFS is up.",
					"Hostname clients need working reverse/forward DNS from the NFS server.",
				},
			},
			{
				Key: "accessMode", Label: "Access", Type: "enum", Default: def.AccessMode, Risk: "medium",
				Summary: "Whether clients may write to the export.",
				Help: "rw allows create/modify/delete. ro rejects writes at the NFS layer (clients still see a filesystem but writes fail). Read-only is safer for media libraries and golden images.",
				Examples: []string{"rw for home directories", "ro for public ISO mirrors"},
				Cases: []string{
					"ro with apps that need caches → applications may fail when writing temp files on the mount.",
					"rw without root_squash → remote root can rewrite any file owned by root on the share.",
				},
				Choices: []Choice{
					{Value: "rw", Label: "Read/Write (rw)", Help: "Allow read and write", Warning: "Clients can modify and delete files on this share."},
					{Value: "ro", Label: "Read-only (ro)", Help: "Allow read only"},
				},
			},
			{
				Key: "commitMode", Label: "Commit", Type: "enum", Default: def.CommitMode, Risk: "medium",
				Summary: "When the server acknowledges writes as durable.",
				Help: "async returns success before data is on stable storage (faster). sync waits for disk (safer). On power loss with async, recent writes can vanish even though the client believed they succeeded.",
				Examples: []string{"async for lab scratch space", "sync for databases / VMs on NFS"},
				Cases: []string{
					"async + sudden reboot → last few seconds (or more) of writes may be lost.",
					"sync on slow disks → client apps feel laggy; throughput drops.",
				},
				Choices: []Choice{
					{Value: "async", Label: "Async", Help: "Faster; risk of loss on crash", Warning: "Recent writes can be lost if the server crashes before flushing to disk."},
					{Value: "sync", Label: "Sync", Help: "Safer; wait for stable storage"},
				},
			},
			{
				Key: "writeDelay", Label: "Write delay", Type: "enum", Default: def.WriteDelay, Risk: "low",
				Summary: "Whether the server batches commits for better throughput.",
				Help: "wdelay (default with sync historically) can delay a WRITE until a related COMMIT arrives, improving throughput. no_wdelay commits sooner, useful for some latency-sensitive workloads. Effect is smaller with async.",
				Examples: []string{"wdelay for general file shares", "no_wdelay when apps issue frequent small syncs"},
				Cases: []string{
					"no_wdelay under heavy small-write load → more disk IOPS, lower latency.",
					"wdelay with poorly behaved clients → occasional longer stalls before COMMIT completes.",
				},
				Choices: []Choice{
					{Value: "wdelay", Label: "wdelay", Help: "Delay writes to improve throughput"},
					{Value: "no_wdelay", Label: "no_wdelay", Help: "Commit immediately when possible"},
				},
			},
			{
				Key: "subtreeCheck", Label: "Subtree check", Type: "enum", Default: def.SubtreeCheck, Risk: "low",
				Summary: "Whether nfsd re-validates that a file handle still lives in the exported tree.",
				Help: "no_subtree_check is the modern recommendation: slightly less strict but avoids many rename/move edge-case failures. subtree_check verifies the file remains under the export path (more CPU, historical security trade-off).",
				Examples: []string{"no_subtree_check (recommended)", "subtree_check only if a policy demands it"},
				Cases: []string{
					"subtree_check + files moved across directories → ESTALE / weird access errors more often.",
					"no_subtree_check is what most current distro defaults use.",
				},
				Choices: []Choice{
					{Value: "no_subtree_check", Label: "no_subtree_check", Help: "Recommended modern default"},
					{Value: "subtree_check", Label: "subtree_check", Help: "Verify file is in exported tree"},
				},
			},
			{
				Key: "portMode", Label: "Ports", Type: "enum", Default: def.PortMode, Risk: "medium",
				Summary: "Whether clients must originate from reserved ports (<1024).",
				Help: "secure requires privileged source ports (traditional UNIX trust model). Most modern clients (macOS, containers, non-root users) need insecure. This server defaults to insecure so typical LAN clients can mount.",
				Examples: []string{"insecure for macOS / Docker / unprivileged mounts", "secure only on locked-down UNIX fleets"},
				Cases: []string{
					"secure + macOS mount → often fails with permission / connection errors.",
					"insecure on hostile networks → any local user can speak NFS as a client (combine with strong client CIDR limits).",
				},
				Choices: []Choice{
					{Value: "insecure", Label: "insecure", Help: "Allow non-reserved client ports", Warning: "Any local process can act as an NFS client; restrict Clients CIDR carefully."},
					{Value: "secure", Label: "secure", Help: "Require reserved ports (<1024)"},
				},
			},
			{
				Key: "lockMode", Label: "Locks", Type: "enum", Default: def.LockMode, Advanced: true, Risk: "low",
				Summary: "Port restrictions specifically for NLM/lock traffic.",
				Help: "insecure_locks mirrors insecure for locking. secure_locks requires reserved ports for lock requests. On NFSv4, locking is integrated; these flags still appear in exportfs -v and matter most for v3-style lock paths.",
				Examples: []string{"insecure_locks with insecure ports", "secure_locks with secure ports"},
				Cases: []string{
					"secure_locks + insecure ports mismatch → some clients take locks fine until a lock-heavy app fails.",
				},
				Choices: []Choice{
					{Value: "insecure_locks", Label: "insecure_locks", Help: "Do not require secure ports for locks"},
					{Value: "secure_locks", Label: "secure_locks", Help: "Require secure ports for locks"},
				},
			},
			{
				Key: "hideMode", Label: "Hide", Type: "enum", Default: def.HideMode, Advanced: true, Risk: "low",
				Summary: "NFSv3 behaviour for subordinate mounts under this export.",
				Help: "On NFSv3, hide prevents clients from seeing nested mount points as part of the parent export; nohide reveals them. This host is NFSv4-only, so the flag is largely cosmetic in exportfs -v, but kept for parity with live export lines.",
				Examples: []string{"hide (default historical)", "nohide if you still care about v3-style nesting"},
				Cases: []string{
					"NFSv4 clients should prefer crossmnt + proper child exports instead of relying on nohide.",
				},
				Choices: []Choice{
					{Value: "hide", Label: "hide", Help: "Hide subordinate mounts (v3)"},
					{Value: "nohide", Label: "nohide", Help: "Do not hide subordinate mounts (v3)"},
				},
			},
			{
				Key: "crossmnt", Label: "Crossmnt", Type: "bool", Default: def.Crossmnt, Risk: "low",
				Summary: "Expose filesystems mounted underneath this export to NFSv4 clients.",
				Help: "When enabled, clients traversing into a bind-mount or secondary mount under the export path can see that content. Without it, they may hit an empty mount point or stop at the boundary. Useful when /mnt has nested mounts.",
				Examples: []string{"off for a single flat share", "on when /mnt/data is a separate bind mount you want visible"},
				Cases: []string{
					"crossmnt off + nested mount → client sees empty directory at the mount point.",
					"crossmnt on without intending to → may expose unexpected volumes under the path.",
				},
			},
			{
				Key: "rootSquash", Label: "Root squash", Type: "enum", Default: def.RootSquash, Risk: "high",
				Summary: "How remote UID 0 (root) is mapped on the server.",
				Help: "root_squash maps remote root to the anonymous UID/GID (safer). no_root_squash preserves remote root as local root — convenient for backups/admin tooling, dangerous if untrusted clients can mount.",
				Examples: []string{"no_root_squash for trusted admin NAS", "root_squash for multi-user shares"},
				Cases: []string{
					"no_root_squash + compromised client → attacker as root on client owns the share as root.",
					"root_squash + tools that need to preserve root-owned files → ownership becomes anonuid (often 65534/nfsnobody).",
				},
				Choices: []Choice{
					{Value: "no_root_squash", Label: "no_root_squash", Help: "Remote root keeps root", Warning: "Remote root can create/chown/delete as local root on this path."},
					{Value: "root_squash", Label: "root_squash", Help: "Map remote root to anon"},
				},
			},
			{
				Key: "allSquash", Label: "All squash", Type: "enum", Default: def.AllSquash, Risk: "medium",
				Summary: "Whether every remote user is forced to the anonymous identity.",
				Help: "all_squash maps all UIDs/GIDs to anonuid/anongid — common for public dropboxes or when clients have mismatched UIDs. no_all_squash keeps normal identity mapping (AUTH_SYS UIDs pass through).",
				Examples: []string{"no_all_squash for home dirs with matching UIDs", "all_squash + anonuid=1000 for a shared drop folder"},
				Cases: []string{
					"all_squash without setting anonuid → files owned by nfsnobody/nobody; clients may lack write permission.",
					"no_all_squash with mismatched UIDs across machines → permission denied or wrong owners.",
				},
				Choices: []Choice{
					{Value: "no_all_squash", Label: "no_all_squash", Help: "Normal UID/GID mapping"},
					{Value: "all_squash", Label: "all_squash", Help: "Map all users to anon", Warning: "Every client identity becomes anonuid/anongid; set those intentionally."},
				},
			},
			{
				Key: "security", Label: "Security flavor", Type: "enum", Default: def.Security, Risk: "medium",
				Summary: "RPC authentication flavor for this export.",
				Help: "sec=sys uses AUTH_SYS (UID/GID in the clear, no crypto). Kerberos flavors (krb5/krb5i/krb5p) are not enabled on this image yet. Combine sec=sys with network trust (VLAN/firewall) and tight Clients lists.",
				Examples: []string{"sec=sys on a private VLAN"},
				Cases: []string{
					"sec=sys on the public internet → trivial to spoof UIDs; do not expose NFS publicly.",
				},
				Choices: []Choice{
					{Value: "sys", Label: "sec=sys", Help: "AUTH_SYS (no cryptography)"},
				},
			},
			{
				Key: "fsid", Label: "Fsid", Type: "number", Default: def.Fsid, Risk: "medium",
				Summary: "Numeric filesystem id; 0 marks the NFSv4 pseudo-root.",
				Help: "Exactly one enabled export must use fsid=0. Other exports need unique positive integers and paths under the fsid=0 directory. Changing fsid on a live share can confuse clients that cached file handles.",
				Examples: []string{"0 for /mnt root", "1 for /mnt/media", "2 for /mnt/backups"},
				Cases: []string{
					"Two exports with fsid=0 → apply rejected.",
					"No fsid=0 → apply rejected.",
					"Child export path not under root path → apply rejected.",
				},
			},
			{
				Key: "anonUid", Label: "anonuid", Type: "number", Advanced: true, Risk: "low",
				Summary: "UID used when squashing (root_squash / all_squash).",
				Help: "Leave empty to use the server default (often 65534). Set explicitly when you want squashed access to match a real local user that owns the directory (e.g. 1000).",
				Examples: []string{"empty (distro default)", "1000", "65534"},
				Cases: []string{
					"all_squash + anonuid=1000 + directory owned by 1000 → clients can write.",
					"anonuid pointing at a non-existent or unwritable owner → permission denied after squash.",
				},
			},
			{
				Key: "anonGid", Label: "anongid", Type: "number", Advanced: true, Risk: "low",
				Summary: "GID used when squashing.",
				Help: "Companion to anonuid. Set both together when using all_squash or relying on root_squash writes into a group-writable tree.",
				Examples: []string{"empty (distro default)", "1000", "65534"},
				Cases: []string{
					"anonuid set but anongid unset → group ownership may still be unexpected.",
				},
			},
			{
				Key: "enabled", Label: "Enabled", Type: "bool", Default: def.Enabled, Risk: "low",
				Summary: "Whether this export is written to /etc/exports on Apply.",
				Help: "Disabled exports stay in the database but are omitted from the live NFS export table until re-enabled and applied.",
				Examples: []string{"enabled for production shares", "disabled to park a config without deleting it"},
				Cases: []string{
					"Disabling the only fsid=0 export → Apply fails until another root is enabled.",
				},
			},
		},
	}
}

func NormalizeExport(e *models.Export) error {
	if e.AccessMode == "" {
		if e.ReadOnly {
			e.AccessMode = "ro"
		} else {
			e.AccessMode = "rw"
		}
	}
	if e.CommitMode == "" {
		if e.Sync {
			e.CommitMode = "sync"
		} else {
			e.CommitMode = "async"
		}
	}
	def := models.DefaultExport()
	if e.WriteDelay == "" {
		e.WriteDelay = def.WriteDelay
	}
	if e.SubtreeCheck == "" {
		e.SubtreeCheck = def.SubtreeCheck
	}
	if e.PortMode == "" {
		e.PortMode = def.PortMode
	}
	if e.LockMode == "" {
		e.LockMode = def.LockMode
	}
	if e.HideMode == "" {
		e.HideMode = def.HideMode
	}
	if e.RootSquash == "" {
		e.RootSquash = def.RootSquash
	}
	if e.AllSquash == "" {
		e.AllSquash = def.AllSquash
	}
	if e.Security == "" {
		e.Security = def.Security
	}

	clients, err := netutil.NormalizeAndValidateClients(e.Clients)
	if err != nil {
		return err
	}
	e.Clients = clients

	e.AccessMode = mustOne(e.AccessMode, "rw", "ro")
	e.CommitMode = mustOne(e.CommitMode, "async", "sync")
	e.WriteDelay = mustOne(e.WriteDelay, "wdelay", "no_wdelay")
	e.SubtreeCheck = mustOne(e.SubtreeCheck, "no_subtree_check", "subtree_check")
	e.PortMode = mustOne(e.PortMode, "insecure", "secure")
	e.LockMode = mustOne(e.LockMode, "insecure_locks", "secure_locks")
	e.HideMode = mustOne(e.HideMode, "hide", "nohide")
	e.RootSquash = mustOne(e.RootSquash, "no_root_squash", "root_squash")
	e.AllSquash = mustOne(e.AllSquash, "no_all_squash", "all_squash")
	e.Security = mustOne(e.Security, "sys")

	e.ReadOnly = e.AccessMode == "ro"
	e.Sync = e.CommitMode == "sync"
	return nil
}

func mustOne(v string, allowed ...string) string {
	v = strings.TrimSpace(v)
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return allowed[0]
}

func BuildExportLine(e models.Export, globalExtra string) (string, error) {
	if err := NormalizeExport(&e); err != nil {
		return "", err
	}
	if strings.TrimSpace(e.Path) == "" || !strings.HasPrefix(e.Path, "/") {
		return "", fmt.Errorf("path must be absolute")
	}

	opts := []string{
		e.AccessMode,
		fmt.Sprintf("fsid=%d", e.Fsid),
		e.CommitMode,
		e.WriteDelay,
		e.HideMode,
		e.SubtreeCheck,
		e.LockMode,
		e.PortMode,
		"sec=" + e.Security,
		e.RootSquash,
		e.AllSquash,
	}
	if e.Crossmnt {
		opts = append(opts, "crossmnt")
	}
	if e.AnonUID != nil {
		opts = append(opts, fmt.Sprintf("anonuid=%d", *e.AnonUID))
	}
	if e.AnonGID != nil {
		opts = append(opts, fmt.Sprintf("anongid=%d", *e.AnonGID))
	}
	for _, extra := range strings.Split(globalExtra, ",") {
		extra = strings.TrimSpace(extra)
		if extra == "" {
			continue
		}
		opts = append(opts, extra)
	}
	return fmt.Sprintf("%s %s(%s)", e.Path, e.Clients, strings.Join(opts, ",")), nil
}

func ValidateExportSet(exports []models.Export) error {
	var root *models.Export
	enabled := make([]models.Export, 0, len(exports))
	for i := range exports {
		e := exports[i]
		if !e.Enabled {
			continue
		}
		if err := NormalizeExport(&e); err != nil {
			return err
		}
		enabled = append(enabled, e)
		if e.Fsid == 0 {
			if root != nil {
				return fmt.Errorf("only one enabled export may use fsid=0 (found %s and %s)", root.Path, e.Path)
			}
			cp := e
			root = &cp
		}
	}
	if len(enabled) == 0 {
		return nil
	}
	if root == nil {
		return fmt.Errorf("exactly one enabled export must use fsid=0 as the NFSv4 root")
	}
	rootPath := path.Clean(root.Path)
	for _, e := range enabled {
		if e.Fsid == 0 {
			continue
		}
		p := path.Clean(e.Path)
		if p == rootPath || !strings.HasPrefix(p, rootPath+"/") {
			return fmt.Errorf("export %s must be a subdirectory of NFSv4 root %s", e.Path, root.Path)
		}
	}
	return nil
}

func ToDTO(e models.Export, globalExtra string) models.ExportDTO {
	_ = NormalizeExport(&e)
	line, _ := BuildExportLine(e, globalExtra)
	return models.ExportDTO{Export: e, ExportLine: line}
}
