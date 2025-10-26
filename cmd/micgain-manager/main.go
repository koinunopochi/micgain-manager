package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"micgain-manager/internal/adapters/volume"
	"micgain-manager/internal/config"
	"micgain-manager/internal/core"
	"micgain-manager/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "serve":
		err = runServe(args)
	case "config":
		err = runConfig(args)
	case "apply":
		err = runApply(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`Mic Gain Manager

USAGE:
  micgain-manager serve [--config path] [--addr host:port]
  micgain-manager config get [--config path]
  micgain-manager config set [--config path] [--volume N] [--interval 45s] [--enable|--disable] [--apply-now]
  micgain-manager apply [--config path] [--volume N]
`)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "設定ファイルのパス")
	addr := fs.String("addr", "127.0.0.1:7070", "HTTPサーバーのbindアドレス")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store, err := config.NewFileStore(*cfgPath)
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

	srv := web.New(mgr, *addr)
	log.Printf("Mic Gain Manager UI: http://%s", *addr)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	return srv.Start()
}

func runConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config サブコマンドとして get か set を指定してください")
	}
	switch args[0] {
	case "get":
		return runConfigGet(args[1:])
	case "set":
		return runConfigSet(args[1:])
	default:
		return fmt.Errorf("不明なconfigサブコマンド: %s", args[0])
	}
}

func runConfigGet(args []string) error {
	fs := flag.NewFlagSet("config get", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "設定ファイルのパス")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := config.NewFileStore(*cfgPath)
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
}

func runConfigSet(args []string) error {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "設定ファイルのパス")
	volumeFlag := fs.Int("volume", -1, "入力音量(0-100)")
	intervalFlag := fs.Duration("interval", 0, "再適用インターバル(e.g. 45s, 2m)")
	enableFlag := fs.Bool("enable", false, "スケジューラを有効化")
	disableFlag := fs.Bool("disable", false, "スケジューラを無効化")
	applyNow := fs.Bool("apply-now", false, "保存後すぐ適用する")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *enableFlag && *disableFlag {
		return errors.New("--enable と --disable は同時に指定できません")
	}
	store, err := config.NewFileStore(*cfgPath)
	if err != nil {
		return err
	}
	cfg, err := store.Load()
	if err != nil {
		return err
	}
	if *volumeFlag >= 0 {
		cfg.TargetVolume = *volumeFlag
	}
	if *intervalFlag > 0 {
		cfg.Interval = *intervalFlag
	}
	if *enableFlag {
		cfg.Enabled = true
	}
	if *disableFlag {
		cfg.Enabled = false
	}
	if updated, err := config.Normalize(cfg); err != nil {
		return err
	} else {
		cfg = updated
	}
	if err := store.Save(cfg); err != nil {
		return err
	}
	fmt.Println("設定を保存しました: volume=", cfg.TargetVolume, " interval=", cfg.Interval, " enabled=", cfg.Enabled)
	if *applyNow {
		fmt.Println("即時適用を実行します…")
		if err := (volume.AppleScriptApplier{}).Apply(cfg.TargetVolume); err != nil {
			return err
		}
		fmt.Println("適用しました")
	}
	return nil
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "設定ファイルのパス")
	volumeFlag := fs.Int("volume", -1, "0-100で指定。未指定なら現在のtargetVolumeを使用")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := config.NewFileStore(*cfgPath)
	if err != nil {
		return err
	}
	cfg, err := store.Load()
	if err != nil {
		return err
	}
	target := cfg.TargetVolume
	if *volumeFlag >= 0 {
		target = *volumeFlag
	}
	fmt.Printf("音量%d%%で適用します…\n", target)
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
}
