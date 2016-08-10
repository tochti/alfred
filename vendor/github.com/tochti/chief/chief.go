package chief

type (
	Job struct {
		Order interface{}
	}

	HandleFunc func(Job)

	PoolChannel chan chan Job

	Worker struct {
		Jobs       chan Job
		Pool       PoolChannel
		HandleFunc HandleFunc
		Quit       chan bool
		QuitDone   chan bool
	}

	Chief struct {
		Jobs       chan Job
		Pool       PoolChannel
		HandleFunc HandleFunc
		Workers    []Worker
		MaxWorkers int
	}
)

// Create a new dispatcher to handle workers
func New(max int, fn HandleFunc) Chief {
	jobs := make(chan Job)
	return Chief{
		Pool:       make(PoolChannel, max),
		Jobs:       jobs,
		HandleFunc: fn,
		Workers:    []Worker{},
		MaxWorkers: max,
	}
}

// Start goroutine to handle jobs
func (c *Chief) Start() {
	for x := 0; x < c.MaxWorkers; x++ {
		w := newWorker(c.Pool, c.HandleFunc)
		c.Workers = append(c.Workers, w)
		w.Start()
	}

	go c.ctrl()
}

// Stop chief, all workers and all channels
func (c Chief) Stop() {
	// close jobs channel due to no one sends any further messages
	close(c.Jobs)

	for _, w := range c.Workers {
		w.Stop()
	}

	close(c.Pool)
}

func (c Chief) ctrl() {
	for {
		select {
		case job, ok := <-c.Jobs:
			if !ok {
				return
			}
			go func(j Job) {
				worker, ok := <-c.Pool
				if !ok {
					return
				}
				worker <- j
			}(job)

		}
	}
}

func newWorker(pool PoolChannel, fn HandleFunc) Worker {
	return Worker{
		Pool:       pool,
		Jobs:       make(chan Job),
		HandleFunc: fn,
		Quit:       make(chan bool),
		QuitDone:   make(chan bool),
	}

}

func (w Worker) Start() {
	go func() {
		for {
			w.Pool <- w.Jobs

			select {
			case job := <-w.Jobs:
				w.HandleFunc(job)

			case <-w.Quit:
				w.QuitDone <- true
				return
			}
		}
	}()
}

func (w Worker) Stop() {
	w.Quit <- true
	close(w.Jobs)
	<-w.QuitDone
}
