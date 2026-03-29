package config

type NotifConfig struct {
	App   NotifAppConfig `yaml:"app"`
	Redis RedisConfig    `yaml:"redis"`
}

type NotifAppConfig struct {
	ID          string `yaml:"id" env:"APP_ID" env-default:"api"`
	Port        string `yaml:"port" env:"APP_PORT" env-default:"8080"`
	Environment string `yaml:"env" env:"APP_ENV" env-default:"development"`

	Backoff BackoffConfig `yaml:"backoff"`
}
