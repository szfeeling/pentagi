package wireless

import (
	"errors"
	"time"
)

var ErrEnvironmentChanged = errors.New("wireless environment changed; explicit revalidation is required")
var ErrConflict = errors.New("wireless state has changed; refresh and try again")
var ErrUnauthorized = errors.New("wireless node authentication failed")

const LeaseDuration = 30 * time.Second

type Area struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	BSSIDs []string `json:"bssids"`
	CIDRs  []string `json:"cidrs"`
}

type Node struct {
	ID              string    `json:"id"`
	AreaID          string    `json:"areaId"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	LastSeen        time.Time `json:"lastSeen"`
	BootID          string    `json:"bootId"`
	PreviousBootIDs []string  `json:"previousBootIds,omitempty"`
	Sequence        int64     `json:"sequence"`
	Capabilities    []string  `json:"capabilities"`
	TokenHash       string    `json:"tokenHash,omitempty"`
}

type Observation struct {
	ID         string    `json:"id"`
	AreaID     string    `json:"areaId"`
	NodeID     string    `json:"nodeId"`
	Kind       string    `json:"kind"`
	MAC        string    `json:"mac"`
	BSSID      string    `json:"bssid"`
	SSID       string    `json:"ssid"`
	Frequency  int       `json:"frequency"`
	Signal     float64   `json:"signal"`
	Security   string    `json:"security"`
	ObservedAt time.Time `json:"observedAt"`
}

type Asset struct {
	ID        string    `json:"id"`
	AreaID    string    `json:"areaId"`
	NodeID    string    `json:"nodeId"`
	Kind      string    `json:"kind"`
	MAC       string    `json:"mac"`
	BSSID     string    `json:"bssid"`
	SSID      string    `json:"ssid"`
	Frequency int       `json:"frequency"`
	Signal    float64   `json:"signal"`
	Security  string    `json:"security"`
	LastSeen  time.Time `json:"lastSeen"`
	Status    string    `json:"status"`
	InScope   bool      `json:"inScope"`
}

type NetworkContext struct {
	ID        string    `json:"id"`
	AreaID    string    `json:"areaId"`
	NodeID    string    `json:"nodeId"`
	BSSID     string    `json:"bssid"`
	Epoch     int64     `json:"epoch"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	DaemonID  string    `json:"daemonId"`
	Network   string    `json:"network"`
	NetworkID string    `json:"networkId"`
	Interface string    `json:"interface"`
	Address   string    `json:"address"`
	Frequency int       `json:"frequency"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Endpoint struct {
	ID        string    `json:"id"`
	ContextID string    `json:"contextId"`
	MAC       string    `json:"mac"`
	IP        string    `json:"ip"`
	Revision  int64     `json:"revision"`
	ExpiresAt time.Time `json:"expiresAt"`
	Status    string    `json:"status"`
}

type Binding struct {
	ContextID        string `json:"contextId"`
	Epoch            int64  `json:"epoch"`
	EndpointID       string `json:"endpointId,omitempty"`
	EndpointRevision int64  `json:"endpointRevision,omitempty"`
}

type FlowBinding struct {
	FlowID   int64         `json:"flowId"`
	Binding  Binding       `json:"binding"`
	Status   string        `json:"status"`
	Evidence *FlowEvidence `json:"evidence,omitempty"`
}

type FlowEvidence struct {
	Context  NetworkContext `json:"context"`
	Endpoint *Endpoint      `json:"endpoint,omitempty"`
	Area     Area           `json:"area"`
	BoundAt  time.Time      `json:"boundAt"`
}

type Job struct {
	ID         string    `json:"id"`
	RequestID  string    `json:"requestId"`
	AreaID     string    `json:"areaId"`
	NodeID     string    `json:"nodeId"`
	Kind       string    `json:"kind"`
	BSSID      string    `json:"bssid"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	LeaseID    string    `json:"leaseId"`
	LeaseUntil time.Time `json:"leaseUntil"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Event struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	SubjectID string    `json:"subjectId"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type Snapshot struct {
	Revision     int64            `json:"revision"`
	Areas        []Area           `json:"areas"`
	Nodes        []Node           `json:"nodes"`
	Assets       []Asset          `json:"assets"`
	Observations []Observation    `json:"observations"`
	Contexts     []NetworkContext `json:"contexts"`
	Endpoints    []Endpoint       `json:"endpoints"`
	Jobs         []Job            `json:"jobs"`
	Flows        []FlowBinding    `json:"flows"`
	Events       []Event          `json:"events"`
}

type Command struct {
	Kind             string   `json:"kind"`
	ExpectedRevision *int64   `json:"expectedRevision,omitempty"`
	RequestID        string   `json:"requestId"`
	AreaID           string   `json:"areaId"`
	NodeID           string   `json:"nodeId"`
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	BSSIDs           []string `json:"bssids"`
	CIDRs            []string `json:"cidrs"`
	JobKind          string   `json:"jobKind"`
	BSSID            string   `json:"bssid"`
}

type Result struct {
	ID    string `json:"id"`
	Token string `json:"token,omitempty"`
}

type JobResult struct {
	ID      string `json:"id"`
	LeaseID string `json:"leaseId"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ContextReport struct {
	ID        string     `json:"id"`
	JobID     string     `json:"jobId"`
	LeaseID   string     `json:"leaseId"`
	BSSID     string     `json:"bssid"`
	DaemonID  string     `json:"daemonId"`
	Network   string     `json:"network"`
	NetworkID string     `json:"networkId"`
	Interface string     `json:"interface"`
	Address   string     `json:"address"`
	Frequency int        `json:"frequency"`
	Connected bool       `json:"connected"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Report struct {
	NodeID       string          `json:"nodeId"`
	BootID       string          `json:"bootId"`
	Sequence     int64           `json:"sequence"`
	Capabilities []string        `json:"capabilities"`
	Observations []Observation   `json:"observations"`
	Contexts     []ContextReport `json:"contexts"`
	Results      []JobResult     `json:"results"`
}

type Dispatch struct {
	Jobs                []Job                `json:"jobs"`
	LeaseUntil          time.Time            `json:"leaseUntil"`
	RevokeContexts      []string             `json:"revokeContexts"`
	VerificationTargets []VerificationTarget `json:"verificationTargets"`
}

type VerificationTarget struct {
	ContextID string    `json:"contextId"`
	MAC       string    `json:"mac"`
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expiresAt"`
}
