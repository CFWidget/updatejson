package main

type ModInfo struct {
	Mods []Mod
}

type Mod struct {
	ModId   string
	Version string
}

type UpdateJson struct {
	Promos   map[string]string `json:"promos"`
	HomePage string            `json:"homepage"`
}
