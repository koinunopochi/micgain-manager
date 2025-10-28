package domain

// ConfigRepository is a secondary port that defines how to persist configuration.
// This interface is defined in the domain layer and implemented by adapters.
type ConfigRepository interface {
	Load() (Config, ScheduleState, error)
	Save(config Config, state ScheduleState) error
}

// VolumeController is a secondary port that defines how to control microphone volume.
// This interface is defined in the domain layer and implemented by adapters.
type VolumeController interface {
	SetVolume(volume int) error
}
