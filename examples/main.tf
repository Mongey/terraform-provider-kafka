provider "kafka" {
  brokers = ["localhost:9092"]
}

resource "kafka_topic" "foo" {
  name               = "moo"
  replication_factor = 1
  partitions         = 3
}

resource "kafka_topic" "foo2" {
  name               = "foo"
  replication_factor = 1
  partitions         = 1

  config = {
    "segment.ms" = "20000"
  }
}
