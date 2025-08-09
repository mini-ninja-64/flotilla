package main

import (
	"github.com/mini-ninja-64/flotilla/internal/ui"
)

func main() {

	progress := ui.MultiLineProgressBars(2)
	go func() {
		for i := 1; i < 5; i++ {
			(progress.Trackers()[0].(ui.ProgressBar)).SetPercentage(float64(i) * 0.25)
			// progress.Finish()
		}
		// progress.SetCompleted()
	}()
	progress.Run()
	// println(err.Error())
	// if err := fang.Execute(context.Background(), root.Cmd()); err != nil {
	// 	os.Exit(1)
	// }
}
