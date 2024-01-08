package worker

import (
	"context"
	"time"

	"github.com/mauricioabreu/mosaic-video/internal/config"
	"github.com/mauricioabreu/mosaic-video/internal/locking"
	"github.com/mauricioabreu/mosaic-video/internal/mosaic"
	"github.com/mauricioabreu/mosaic-video/internal/watcher"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Run(lc fx.Lifecycle, cfg *config.Config, logger *zap.SugaredLogger, locker *locking.RedisLocker, fsw *watcher.FileSystemWatcher) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				runningProcesses := make(map[string]bool)

				if err := fsw.Start(); err != nil {
					logger.Fatal(err)
				}

				go func() {
					for event := range fsw.Events() {
						logger.Infof("File system event: %v", event)
					}
				}()

				for {
					logger.Info("worker is running")

					tasks, err := mosaic.FetchMosaicTasks(cfg.API.URL)
					if err != nil {
						logger.Fatal(err)
					}

					for _, task := range tasks {
						go func(m mosaic.Mosaic) {
							defer func() {
								// Once finished, unmark the task
								delete(runningProcesses, m.Name)
							}()

							if err := GenerateMosaic(m, cfg, locker, &mosaic.FFMPEGCommand{}, runningProcesses); err != nil {
								logger.Error(err)
							}
						}(task)
					}

					time.Sleep(60 * time.Second)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			fsw.Stop()
			return nil
		},
	})
}
