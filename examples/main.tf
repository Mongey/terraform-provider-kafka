provider "kafka" {
  brokers = ["localhost:9092"]
}

resource "kafka_topic" "syslog" {
  name               = "syslog"
  replication_factor = 1
  partitions         = 200

  config = {
    "segment.ms"   = "4000"
    "retention.ms" = "86400000"
  }
}
