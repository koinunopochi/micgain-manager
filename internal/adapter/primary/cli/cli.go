package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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
		newShellCmd(),
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

func newShellCmd() *cobra.Command {
	var prompt string
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Cobraサブコマンドを対話的に叩けるシェルを起動",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractiveShell(prompt)
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "micgain> ", "シェルのプロンプト文字列")
	return cmd
}

func runInteractiveShell(prompt string) error {
	historyFile := filepath.Join(os.TempDir(), "micgain-manager-shell.history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     historyFile,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	sessionVerbosity := verbosity
	fmt.Println("対話型シェルを開始します。'help' で使い方、'exit' で終了。")

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			fmt.Println()
			continue
		}
		if err == io.EOF {
			fmt.Println()
			return nil
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch line {
		case "exit", "quit":
			fmt.Println("Bye!")
			return nil
		case "help":
			printShellHelp()
			continue
		}
		tokens, err := shlex.Split(line)
		if err != nil {
			fmt.Printf("Parse error: %v\n", err)
			continue
		}
		if len(tokens) == 0 {
			continue
		}
		if tokens[0] == "log" {
			if err := handleShellLog(tokens[1:], &sessionVerbosity); err != nil {
				fmt.Printf("log: %v\n", err)
			}
			continue
		}
		if tokens[0] == "shell" {
			fmt.Println("すでにシェル内です。他のコマンドを入力するか 'exit' で終了してください。")
			continue
		}

		verbosity = sessionVerbosity
		if err := executeArgs(tokens); err != nil {
			fmt.Printf("command error: %v\n", err)
		}
		sessionVerbosity = verbosity
	}
}

func executeArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	root := NewRootCmd()
	root.SetArgs(args)
	return root.Execute()
}

func handleShellLog(args []string, sessionVerbosity *int) error {
	fs := pflag.NewFlagSet("log", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var vcount int
	var level string
	var show bool
	fs.CountVarP(&vcount, "verbose", "v", "Increase verbosity (-v... up to 4)")
	fs.StringVar(&level, "level", "", "指定レベル(error|warn|info|debug|trace)")
	fs.BoolVarP(&show, "show", "s", false, "現在のレベルを表示")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch {
	case show && vcount == 0 && level == "":
		fmt.Printf("log level: %s (-v x%d)\n", logging.LevelName(), logging.Verbosity())
		return nil
	case level != "":
		_, count, err := logging.ParseLevel(level)
		if err != nil {
			return err
		}
		*sessionVerbosity = count
	case vcount > 0:
		*sessionVerbosity = vcount
	default:
		fmt.Printf("log level: %s (-v x%d)\n", logging.LevelName(), logging.Verbosity())
		return nil
	}

	verbosity = *sessionVerbosity
	logging.SetVerbosity(*sessionVerbosity)
	fmt.Printf("log level set to %s (-v x%d)\n", logging.LevelName(), logging.Verbosity())
	return nil
}

func printShellHelp() {
	fmt.Println(`利用可能な入力例:
  daemon                      # スケジューラを起動
  web --addr 0.0.0.0:7070     # Web UIを起動
  serve --addr 0.0.0.0:8080   # Web UI + スケジューラを起動
  config get                  # 設定を確認
  config set --volume 70      # 設定を更新
  apply --volume 45           # 即時適用のみ実施
  log -vv                     # ログ出力を詳細化
  log --show                  # 現在のログレベルを確認
  exit / quit                 # シェル終了`)
}
