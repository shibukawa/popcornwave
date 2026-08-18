package handlers

// The shared configuration layer rather than a runtime: this file belongs to
// both builds, and binding a setting is not a transport concern.
import "github.com/shibukawa/popcornweb/pwconfig"

// AppConfig is an application-owned configuration binding. Its values come from
// the [app] table of config.{APP_ENV}.toml, from APP_ENV_LABEL and
// APP_ENV_LABEL_COLOR, or from --app-env_label and --app-env_label_color.
type AppConfig struct {
	EnvLabel      string `default:"local" help:"environment name shown in the page badge"`
	EnvLabelColor string `default:"#64748b" help:"CSS color of the environment badge"`
}

// RegisterConfig binds AppConfig to the "app" prefix. Call it from main: the
// generated definition registers during package init, so the binding itself
// must be created after all init functions have run.
func RegisterConfig() { pwconfig.Register[AppConfig]("app") }
