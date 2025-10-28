package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"micgain-manager/internal/adapter/primary/web"
	"micgain-manager/internal/adapter/secondary/repository"
	"micgain-manager/internal/adapter/secondary/volume"
	"micgain-manager/internal/logging"
	"micgain-manager/internal/usecase"
)

var (
	cfgPath   string
	verbosity int
)

// NewRootCmd creates the root CLI command.
// This is the primary adapter that translates CLI inputs to use case calls.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "micgain-manager",
		Short: "macOSのマイク入力音量を固定するCLI/Webサーバー",
		Long:  "Scheduler + Web UI + CLIを兼ねるマイク入力ゲイン固定ツール",
	}

	defaultCfg := repository.DefaultPath()
	cmd.PersistentFlags().StringVar(&cfgPath, "config", defaultCfg, "設定ファイルのパス")
	cmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "ロギングを詳細化 (-v, -vv, ... 最大4回)")
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		logging.SetVerbosity(verbosity)
	}

	cmd.AddCommand(
		newDaemonCmd(),
		newWebCmd(),
		newServeCmd(),
		newConfigCmd(),
		newApplyCmd(),
	)

	return cmd
}

func newDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "スケジューラのみを起動（Webサーバーなし）",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repository.NewFileRepository(cfgPath)
			if err != nil {
				return err
			}
			controller := volume.NewAppleScriptController()
			uc, err := usecase.NewSchedulerUseCase(repo, controller)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			fmt.Println("Mic Gain Manager daemon started")
			logging.Infof("Scheduler daemon started")
			uc.Start(ctx)

			<-ctx.Done()
			fmt.Println("Daemon shutting down...")
			return nil
		},
	}
}

func newWebCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Web UIとREST APIのみを起動（スケジューラなし）",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repository.NewFileRepository(cfgPath)
			if err != nil {
				return err
			}
			controller := volume.NewAppleScriptController()
			uc, err := usecase.NewSchedulerUseCase(repo, controller)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			srv := web.NewServer(uc, addr)
			fmt.Printf("Mic Gain Manager Web UI running at http://%s\n", addr)
			logging.Infof("Web UI: http://%s (scheduler disabled)", addr)

			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()

			return srv.Start()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7070", "HTTPサーバーのアドレス:ポート")
	return cmd
}

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Web UIとスケジューラを両方起動",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repository.NewFileRepository(cfgPath)
			if err != nil {
				return err
			}
			controller := volume.NewAppleScriptController()
			uc, err := usecase.NewSchedulerUseCase(repo, controller)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			// Start scheduler
			uc.Start(ctx)

			srv := web.NewServer(uc, addr)
			fmt.Printf("Mic Gain Manager UI running at http://%s\n", addr)
			logging.Infof("Mic Gain Manager UI: http://%s", addr)

			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()

			return srv.Start()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7070", "HTTPサーバーのアドレス:ポート")
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "設定の取得・更新を行うサブコマンド",
	}
	cmd.AddCommand(newConfigGetCmd(), newConfigSetCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "現在の設定(JSON)を表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repository.NewFileRepository(cfgPath)
			if err != nil {
				return err
			}
			config, state, err := repo.Load()
			if err != nil {
				return err
			}

			// Convert to display format
			display := map[string]interface{}{
				"targetVolume":    config.TargetVolume,
				"intervalSeconds": int(config.Interval.Seconds()),
				"enabled":         config.Enabled,
				"lastApplyStatus": state.LastApplyStatus.String(),
			}
			if !state.LastApplied.IsZero() {
				display["lastApplied"] = state.LastApplied.Format(time.RFC3339)
			}
			if state.LastError != nil {
				display["lastError"] = state.LastError.Error()
			}

			out, _ := json.MarshalIndent(display, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	var (
		volumeFlag   int
		intervalFlag time.Duration
		enabledFlag  string
		applyNow     bool
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "設定を書き換え(必要なら即時適用)",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repository.NewFileRepository(cfgPath)
			if err != nil {
				return err
			}
			controller := volume.NewAppleScriptController()
			uc, err := usecase.NewSchedulerUseCase(repo, controller)
			if err != nil {
				return err
			}

			snapshot := uc.GetSnapshot()
			config := snapshot.Config

			if cmd.Flags().Changed("volume") {
				config.TargetVolume = volumeFlag
			}
			if cmd.Flags().Changed("interval") {
				config.Interval = intervalFlag
			}
			if cmd.Flags().Changed("enabled") {
				switch enabledFlag {
				case "true":
					config.Enabled = true
				case "false":
					config.Enabled = false
				default:
					return errors.New("--enabled には true/false を指定してください")
				}
			}

			if err := uc.UpdateConfig(config, applyNow); err != nil {
				return err
			}

			fmt.Printf("保存しました: volume=%d interval=%s enabled=%t\n",
				config.TargetVolume, config.Interval, config.Enabled)
			if applyNow {
				fmt.Println("適用完了")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&volumeFlag, "volume", 50, "入力音量(0-100)")
	cmd.Flags().DurationVar(&intervalFlag, "interval", time.Minute, "再適用インターバル 例:45s,2m")
	cmd.Flags().StringVar(&enabledFlag, "enabled", "", "true/false を指定するとスケジューラON/OFF")
	cmd.Flags().BoolVar(&applyNow, "apply-now", false, "保存後ただちに適用")
	return cmd
}

func newApplyCmd() *cobra.Command {
	var volumeFlag int
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "現在の設定または指定音量で即時適用",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repository.NewFileRepository(cfgPath)
			if err != nil {
				return err
			}
			controller := volume.NewAppleScriptController()
			uc, err := usecase.NewSchedulerUseCase(repo, controller)
			if err != nil {
				return err
			}

			volume := -1
			if cmd.Flags().Changed("volume") {
				volume = volumeFlag
			}

			fmt.Printf("音量適用中...\n")
			if err := uc.ApplyNow(volume); err != nil {
				return err
			}
			fmt.Println("完了")
			return nil
		},
	}
	cmd.Flags().IntVar(&volumeFlag, "volume", 0, "0-100を指定。未指定なら設定値を利用")
	return cmd
}
