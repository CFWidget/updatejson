package models

type Mod struct {
	Id           string
	Version      string
	Dependencies []string
}

type UpdateJson struct {
	Promos     map[string]string `json:"promos"`
	References References        `json:"-"`
	HomePage   string            `json:"homepage"`
}

type References map[string]string
