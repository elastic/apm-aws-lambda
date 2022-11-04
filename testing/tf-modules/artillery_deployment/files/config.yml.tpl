config:
  target: ${load_base_url}
  processor: "./coldstart-metrics.js"
  phases:
    - duration: ${load_duration}
      arrivalRate: ${load_arrival_rate}

scenarios:
  - name: get
    afterResponse: "generateColdstartAwareMetrics"
    flow:
      - get:
          url: ${load_req_path}
