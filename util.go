package main

func RunSeries(fns ...func() error) (err error) {
	for _, fn := range fns {
		if err = fn(); err != nil {
			return
		}
	}

	return
}
