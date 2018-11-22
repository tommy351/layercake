package main

func runSeries(fns ...func() error) (err error) {
	for _, fn := range fns {
		if err = fn(); err != nil {
			return
		}
	}

	return
}
