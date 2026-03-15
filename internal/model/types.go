package model

type ModelConfig struct {
	ModelName           string         `json:"model_name" yaml:"model_name"`
	ModelKwargs         map[string]any `json:"model_kwargs" yaml:"model_kwargs"`
	FormatErrorTemplate string         `json:"format_error_template" yaml:"format_error_template"`
	ObservationTemplate string         `json:"observation_template" yaml:"observation_template"`
}
