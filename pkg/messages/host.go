package messages

// Host is the host details
type Host struct {
	IncomingHost   string
	Namespace      string
	SourceService  string
	TargetService  string
	SourceHost     string
	TargetHost     string
	TrafficAllowed bool
}
