package core

import (
	"fmt"
	"time"

	"micgain-manager/internal/config"
)

// HandleEvent is a pure function that takes current state and an event,
// and returns the new state along with effects to be executed.
// This function has no side effects and is fully testable.
func HandleEvent(state State, event Event, now time.Time) (State, []Effect, error) {
	switch event.Type {
	case EventTick:
		return handleTick(state, now)
	case EventUpdateConfig:
		data, ok := event.Data.(UpdateConfigData)
		if !ok {
			return state, nil, fmt.Errorf("invalid UpdateConfigData")
		}
		return handleUpdateConfig(state, data, now)
	case EventApplyOnce:
		data, ok := event.Data.(ApplyOnceData)
		if !ok {
			return state, nil, fmt.Errorf("invalid ApplyOnceData")
		}
		return handleApplyOnce(state, data, now)
	default:
		return state, nil, fmt.Errorf("unknown event type: %s", event.Type)
	}
}

func handleTick(state State, now time.Time) (State, []Effect, error) {
	if !state.Config.Enabled {
		newState := state
		newState.NextRun = time.Time{}
		newState.RunningSince = nil
		return newState, nil, nil
	}

	if !now.After(state.NextRun) && !state.NextRun.IsZero() {
		return state, nil, nil
	}

	effects := []Effect{
		{
			Type:   EffectApplyVolume,
			Volume: state.Config.TargetVolume,
		},
	}

	newConfig := state.Config
	newConfig.LastApplied = now
	newConfig.LastApplyStatus = "ok"
	newConfig.LastError = ""

	effects = append(effects, Effect{
		Type:   EffectSaveConfig,
		Config: newConfig,
	})

	runningSince := now
	newState := State{
		Config:       newConfig,
		NextRun:      now.Add(newConfig.Interval),
		RunningSince: &runningSince,
	}

	return newState, effects, nil
}

func handleUpdateConfig(state State, data UpdateConfigData, now time.Time) (State, []Effect, error) {
	normalized, err := config.Normalize(data.Config)
	if err != nil {
		return state, nil, err
	}

	effects := []Effect{
		{
			Type:   EffectSaveConfig,
			Config: normalized,
		},
	}

	if data.ApplyNow {
		effects = append(effects, Effect{
			Type:   EffectApplyVolume,
			Volume: normalized.TargetVolume,
		})
	}

	newState := State{
		Config:       normalized,
		NextRun:      now.Add(normalized.Interval),
		RunningSince: nil,
	}

	return newState, effects, nil
}

func handleApplyOnce(state State, data ApplyOnceData, now time.Time) (State, []Effect, error) {
	volume := data.Volume
	if volume < 0 {
		volume = state.Config.TargetVolume
	}

	effects := []Effect{
		{
			Type:   EffectApplyVolume,
			Volume: volume,
		},
	}

	newConfig := state.Config
	newConfig.LastApplied = now
	newConfig.LastApplyStatus = "ok"
	newConfig.LastError = ""

	effects = append(effects, Effect{
		Type:   EffectSaveConfig,
		Config: newConfig,
	})

	runningSince := now
	newState := State{
		Config:       newConfig,
		NextRun:      state.NextRun,
		RunningSince: &runningSince,
	}

	return newState, effects, nil
}

// HandleEffectResult updates state based on the result of executing an effect.
// This is called after an effect has been executed to update the state accordingly.
func HandleEffectResult(state State, effect Effect, err error) State {
	if err == nil {
		// Effect succeeded, clear RunningSince
		newState := state
		newState.RunningSince = nil
		return newState
	}

	// Effect failed, update error status
	newConfig := state.Config
	newConfig.LastApplyStatus = "error"
	newConfig.LastError = err.Error()

	newState := state
	newState.Config = newConfig
	newState.RunningSince = nil

	return newState
}
