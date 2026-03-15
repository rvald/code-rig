package environment

type LocalEnvironmentConfig struct {
	Cwd     string
	Env     map[string]string
	Timeout int
}
