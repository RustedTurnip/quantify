package main

import (
	monitoring "cloud.google.com/go/monitoring/apiv3"
	"context"
	"google.golang.org/api/option"
	"log"
	"math/rand"
	"quantify"
	"time"
)

func main() {

	ctx := context.Background()

	// google cloud monitoring client
	m, err := monitoring.NewMetricClient(ctx, option.WithCredentialsFile("/path/to/file.json"))

	// Quantify client
	cli, err := quantify.New(
		ctx,
		quantify.OptionWithCloudMetricsClient(m),
		quantify.OptionWithResourceType(&quantify.Global{
			ProjectId: "quantify",
		}),
		quantify.OptionWithErrorHandler(func(quantifier *quantify.Quantifier, err error) {
			log.Fatal(err)
		}),
	)
	if err != nil {
		panic(err)
	}

	// create counters
	b738Counter, err := cli.CreateCounter(
		"planes",
		map[string]string{
			"manufacturer": "boeing",
			"model":        "737-800",
		},
		10,
	)
	if err != nil {
		panic(err)
	}

	b739Counter, err := cli.CreateCounter(
		"planes",
		map[string]string{
			"manufacturer": "boeing",
			"model":        "737-900",
		},
		10,
	)
	if err != nil {
		panic(err)
	}

	// random count
	t := time.NewTicker(time.Second * 10)
	count := 0

	for range t.C {

		count++

		if count < 6 {

			for i := 0; i < rand.Intn(500); i++ {
				b738Counter.Count()
			}

			for i := 0; i < rand.Intn(500); i++ {
				b739Counter.Count()
			}

			continue
		}

		t.Stop()
		break
	}

	// cease counting
	cli.Stop()
}
