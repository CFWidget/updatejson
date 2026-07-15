package models

type TomlModInfo struct {
	Mods         []TomlMod
	Dependencies map[string][]Dependency
}

type TomlMod struct {
	ModId   string
	Version string
}

type JsonMod struct {
	ModId             string `json:"id"`
	Version           string `json:"version"`
	LegacyFormatModId string `json:"modid"`
}

type Dependency struct {
	ModId string
}

type McMod struct {
	ModList []JsonMod
}
