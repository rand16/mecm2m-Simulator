package m2mapi

type Area struct {
	//data property
	AreaID      string
	Address     string
	NE          SquarePoint
	SW          SquarePoint
	Description string
	// Name string
	// IoTSPIDs []string

	//object property
	//contains PSink
}

type PSink struct {
	//data property
	VPointID_n  string
	Address     string
	Lat         float64
	Lon         float64
	Description string
	VPointID    string //追加
	ServerIP    string //追加
	// Policy         string
	// IoTSPID        string
}

type ResolvePoint struct {
	//input
	NE SquarePoint
	SW SquarePoint

	//output
	VPointID_n string
	Address    string
}

type ResolveNode struct {
	//input
	VPointID_n string
	CapsInput  []string

	//output
	VNodeID_n string
	CapOutput string
}

type SquarePoint struct {
	Lat float64
	Lon float64
}
