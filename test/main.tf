provider "kafka" {
  address = "localhost:9092"
}

resource "kafka_topic" "foo" {
  name               = "moo7"
  replication_factor = 1
  partitions         = 3
}
