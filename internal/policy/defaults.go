package policy

import _ "embed"

//go:embed defaults/private.toml
var privateDefaults []byte

//go:embed defaults/public.toml
var publicDefaults []byte

// PersonalDefaults selects a complete preset after repository visibility is known.
// Explicit TOML configurations never inherit these values.
func PersonalDefaults() Config              { return Config{personalDefaults: true} }
func (c Config) UsesPersonalDefaults() bool { return c.personalDefaults }
func (c Config) ForTarget(t Target) (Config, error) {
	if !c.personalDefaults {
		return c, nil
	}
	if t.Private {
		return Parse("built-in/private.toml", privateDefaults)
	}
	return Parse("built-in/public.toml", publicDefaults)
}
