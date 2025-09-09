package handler

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"
)

func (h *Handler) ChangeDriverStatus(ctx context.Context, b *bot.Bot) {
	h.logger.Info("statarted change driver status service")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				h.logger.Info("driver status service canceled")
				return
			}
		case <-ticker.C:
			ids, err := h.driverRepo.ChangeDriverStatus(ctx, "pending", "approved")
			if err != nil {
				h.logger.Error("change driver status error", zap.Error(err))
			}
			for i := 0; i < len(ids); i++ {
				id := ids[i]
				_, err := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: id,
					Text:   "✅ Сіздің жүргізуші мәртебеңіз мақұлданды! 🚗 Енді сіз жолсапарды бастай аласыз! 🎉🛣️",
				})
				if err != nil {
					h.logger.Error("error send message to driver", zap.Error(err))
				}
			}
		}
	}
}
