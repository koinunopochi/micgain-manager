package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"micgain-manager/internal/adapters/volume"
	"micgain-manager/internal/config"
	"micgain-manager/internal/core"
	"micgain-manager/internal/web"
)

var cfgPath string

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "micgain-manager",
		Short: "macOSのマイク入力音量を固定するCLI/Webサーバー",
		Long:  "Scheduler + Web UI + CLIを兼ねるマイク入力ゲイン固定ツール",
	}
	cmd.PersistentFlags().StringVar(&cfgPath, "config", config.DefaultPath(), "設定ファイルのパス")
	cmd.AddCommand(newServeCmd(), newConfigCmd(), newApplyCmd())
	return cmd
}

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Web UIとREST APIを含む永続サーバーを起動",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewFileStore(cfgPath)
			if err != nil {
				return err
			}
			mgr, err := core.NewManager(store, volume.AppleScriptApplier{})
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			mgr.Start(ctx)

			srv := web.New(mgr, addr)
			log.Printf("Mic Gain Manager UI: http://%s", addr)
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
			store, err := config.NewFileStore(cfgPath)
			if err != nil {
				return err
			}
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(cfg, "", "  ")
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
			store, err := config.NewFileStore(cfgPath)
			if err != nil {
				return err
			}
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("volume") {
				cfg.TargetVolume = volumeFlag
			}
			if cmd.Flags().Changed("interval") {
				cfg.Interval = intervalFlag
			}
			if cmd.Flags().Changed("enabled") {
				switch enabledFlag {
				case "true":
					cfg.Enabled = true
				case "false":
					cfg.Enabled = false
				default:
					return errors.New("--enabled には true/false を指定してください")
				}
			}
			if cfg, err = config.Normalize(cfg); err != nil {
				return err
			}
			if err := store.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("保存しました: volume=%d interval=%s enabled=%t\n", cfg.TargetVolume, cfg.Interval, cfg.Enabled)
			if applyNow {
				fmt.Println("即時適用中…")
				if err := (volume.AppleScriptApplier{}).Apply(cfg.TargetVolume); err != nil {
					return err
				}
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
			store, err := config.NewFileStore(cfgPath)
			if err != nil {
				return err
			}
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			target := cfg.TargetVolume
			if cmd.Flags().Changed("volume") {
				target = volumeFlag
			}
			fmt.Printf("音量%d%%で適用しています…\n", target)
			if err := (volume.AppleScriptApplier{}).Apply(target); err != nil {
				return err
			}
			cfg.LastApplied = time.Now()
			cfg.LastApplyStatus = "ok"
			cfg.LastError = ""
			if err := store.Save(cfg); err != nil {
				return err
			}
			fmt.Println("完了")
			return nil
		},
	}
	cmd.Flags().IntVar(&volumeFlag, "volume", 0, "0-100を指定。未指定なら設定値を利用")
	return cmd
}
