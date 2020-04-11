package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
)

func traceF1(tracer opentracing.Tracer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		spanCtx, err := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
		if err != nil {
			log.Println(err)
		}
		span := tracer.StartSpan("start", ext.RPCServerOption(spanCtx))
		defer span.Finish()

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		ctx = opentracing.ContextWithSpan(ctx, span)
		s, return1 := f1(ctx)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, s+return1)
		return
	}
}

func f1(ctx context.Context) (string, string) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "f1")
	defer span.Finish()

	sleept := time.Duration(rand.Intn(1120)) * time.Millisecond
	span.LogKV("sleep", sleept)

	time.Sleep(sleept)

	return1 := f2(ctx)

	s := "f1 done:"

	return s, return1

}

func f2(ctx context.Context) string {
	span, ctx := opentracing.StartSpanFromContext(ctx, "f2")
	defer span.Finish()

	sleept := time.Duration(rand.Intn(1920)) * time.Millisecond
	span.LogKV("sleep", sleept)

	time.Sleep(sleept)

	return "f2 done:"
}

// ----------

func main() {

	log.Println("starting...")

	cfg := jaegercfg.Configuration{
		ServiceName: "app2",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}

	jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory

	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)

	if err != nil {
		log.Fatalf("could not initialize jaeger tracer: %s", err.Error())
	}
	defer closer.Close()

	opentracing.SetGlobalTracer(tracer)

	f1 := traceF1(tracer)
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
		return
	})
	http.HandleFunc("/", f1)

	log.Fatalln(http.ListenAndServe(":8181", nil))
}
