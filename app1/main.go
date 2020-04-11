package main

import (
	"context"
	"fmt"
	"io/ioutil"
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

		spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
		span := tracer.StartSpan("start", ext.RPCServerOption(spanCtx))
		defer span.Finish()

		span.Context().ForeachBaggageItem(func(k, v string) bool {
			fmt.Println(span, "baggage:", k, v)
			span.LogKV(k, v)
			return true
		})

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		ctx = opentracing.ContextWithSpan(ctx, span)

		s, return1, return2 := f1(ctx, tracer)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, s+return1+return2)
		return
	}
}

func f1(ctx context.Context, tracer opentracing.Tracer) (string, string, string) {

	// span := root.Tracer().StartSpan("f1", opentracing.ChildOf(root.Context()))
	span, ctx := opentracing.StartSpanFromContext(ctx, "f1")
	defer span.Finish()

	sleept := time.Duration(rand.Intn(1120)) * time.Millisecond
	time.Sleep(sleept)

	return1 := f2(ctx)
	return2 := f3(ctx, tracer)

	s := "<html><body>f1 done:"

	return s, return1, return2

}

func f2(ctx context.Context) string {
	span, ctx := opentracing.StartSpanFromContext(ctx, "f2")
	defer span.Finish()

	sleept := time.Duration(rand.Intn(1920)) * time.Millisecond
	span.LogKV("sleep", sleept)
	time.Sleep(sleept)

	return "f2 done:"
}

func f3(ctx context.Context, tracer opentracing.Tracer) string {
	span, ctx := opentracing.StartSpanFromContext(ctx, "f3")
	defer span.Finish()

	url := "http://localhost:8181/"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		ext.Error.Set(span, true)
		span.LogKV("error", err)
		return err.Error()
	}

	span.LogKV("request", req.URL)

	ext.SpanKindRPCClient.Set(span)
	ext.PeerAddress.Set(span, url)
	ext.PeerService.Set(span, "app2")

	ext.HTTPUrl.Set(span, url)
	ext.HTTPMethod.Set(span, "GET")

	client := &http.Client{Timeout: time.Second * 10}
	tracer.Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))

	response, err := client.Do(req)
	if err != nil {
		span.LogKV("error", err)
		ext.Error.Set(span, true)
		log.Println(err)
		return err.Error()
	}
	defer response.Body.Close()

	out, err := ioutil.ReadAll(response.Body)
	if err != nil {
		ext.Error.Set(span, true)
		span.LogKV("error", err)
		log.Println(err)
		return err.Error()
	}

	return "f3 done: <br>&nbsp;&nbsp; => " + string(out) + "</body></html>"
}

// ----------

func main() {

	log.Println("starting...")

	cfg := jaegercfg.Configuration{
		ServiceName: "app1",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1, // trace every call
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: false,
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

	log.Fatalln(http.ListenAndServe(":8080", nil))
}
