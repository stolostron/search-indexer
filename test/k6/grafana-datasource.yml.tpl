apiVersion: 1
datasources:
  - name: k6
    uid: k6
    type: influxdb
    access: proxy
    url: http://influxdb:8086
    database: k6
    isDefault: true
  - name: cluster-prometheus
    uid: cluster-prometheus
    type: prometheus
    access: proxy
    url: https://${THANOS_HOST}
    isDefault: false
    jsonData:
      httpHeaderName1: Authorization
      tlsSkipVerify: true
    secureJsonData:
      httpHeaderValue1: Bearer ${API_TOKEN}
