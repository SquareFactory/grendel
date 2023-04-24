package watch

// grendel host.json config struct
type Host struct {
	Name       string `json:"name"`
	Provision  bool   `json:"provision"`
	BootImage  string `json:"boot_image"`
	Interfaces []struct {
		IP  string `json:"ip"`
		Mac string `json:"mac"`
		Bmc bool   `json:"bmc"`
	} `json:"interfaces"`
}

// grendel image.json config struct
type Image struct {
	Name    string   `json:"name"`
	Kernel  string   `json:"kernel"`
	Initrd  []string `json:"initrd"`
	Liveimg string   `json:"liveimg"`
	Cmdline string   `json:"cmdline"`
}
