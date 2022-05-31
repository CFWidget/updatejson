package main

type ModInfo struct {
	Mods []Mod
}

type Mod struct {
	ModId   string `json:"id"`
	Version string `json:"version"`
}

type UpdateJson struct {
	Promos   map[string]string `json:"promos"`
	HomePage string            `json:"homepage"`
}

type References map[string]string
