package lima

import "time"

type InstanceStatus string

const (
	StatusRunning InstanceStatus = "Running"
	StatusStopped InstanceStatus = "Stopped"
	StatusUnknown InstanceStatus = ""
)

type Instance struct {
	Name    string         `json:"name"`
	Status  InstanceStatus `json:"status"`
	Dir     string         `json:"dir"`
	Arch    string         `json:"arch"`
	CPUs    int            `json:"cpus"`
	Memory  int64          `json:"memory"`
	Disk    int64          `json:"disk"`
	SSHAddr string         `json:"sshAddress"`
	Network []NetworkInfo  `json:"network"`
}

type NetworkInfo struct {
	VNL       string `json:"vnl"`
	Interface string `json:"interface"`
	MACAddr   string `json:"macAddress"`
	IPAddr    string `json:"ipAddress"`
}

type CreateOptions struct {
	Name     string
	Template string // path to YAML template
	CPUs     int
	Memory   string // e.g. "3GiB"
	Disk     string // e.g. "30GiB"
	Start    bool
	Timeout  time.Duration
}

type CloneOptions struct {
	Source  string
	Target  string
	Start   bool
	Timeout time.Duration
}

type CopyDirection int

const (
	CopyToVM CopyDirection = iota
	CopyFromVM
)

type CopyOptions struct {
	Instance  string
	Direction CopyDirection
	LocalPath string
	VMPath    string
}

type ShellOptions struct {
	Instance string
	Command  string
	Args     []string
	Timeout  time.Duration
}
