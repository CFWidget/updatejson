package main

type ModInfo struct {
	Mods      []Mod
	ModLoader string
}

type Mod struct {
	ModId    string `json:"id"`
	Version  string `json:"version"`
	OldModId string `json:"modid"`
}

type McMod struct {
	ModList []Mod
}

type UpdateJson struct {
	Promos     map[string]string `json:"promos"`
	References References        `json:"-"`
	HomePage   string            `json:"homepage"`
}

type References map[string]string
