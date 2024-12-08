resource "kubernetes_namespace" "backend-kafka-stack-ns" {
  metadata {
    name = "backend-kafka-stack"
  }
}


resource "helm_release" "kafka-deployment" {
  name       = "kafka-helm"
  chart      = "../helm/kafka"

  depends_on = [kubernetes_namespace.backend-kafka-stack-ns]

}

resource "null_resource" "kafka-topic" {
  depends_on = [helm_release.kafka-deployment]

  provisioner "local-exec" {
    command = <<EOT
    kubectl exec -n backend-kafka-stack $(kubectl get pods -n backend-kafka-stack -l app=kafka -o jsonpath='{.items[0].metadata.name}') -- kafka-topics.sh --create --topic user-logs --bootstrap-server kafka:9092 --partitions 1 --replication-factor 1
    EOT
  }
}

resource "helm_release" "backend-deployment" {
  name       = "backend-helm"
  chart      = "../helm/backend"

  depends_on = [null_resource.kafka-topic]

}


resource "null_resource" "mongodb-setup-create-db" {
  depends_on = [helm_release.backend-deployment]

  provisioner "local-exec" {
    command = <<EOT
    kubectl exec -n backend-kafka-stack $(kubectl get pods -n backend-kafka-stack -l app=mongodb -o jsonpath='{.items[0].metadata.name}') -- mongosh -u root -p root --eval "use users;"
    EOT
  }
}

resource "null_resource" "mongodb-setup-create-collection" {
  depends_on = [null_resource.mongodb-setup-create-db]

  provisioner "local-exec" {
    command = <<EOT
    kubectl exec -n backend-kafka-stack $(kubectl get pods -n backend-kafka-stack -l app=mongodb -o jsonpath='{.items[0].metadata.name}') -- mongosh -u root -p root --eval "db.createCollection('users');"
    EOT
  }
}


resource "null_resource" "kafka-topic-watching" {
  depends_on = [null_resource.mongodb-setup-create-collection]

  provisioner "local-exec" {
    command = <<EOT
    kubectl exec -n backend-kafka-stack $(kubectl get pods -n backend-kafka-stack -l app=kafka -o jsonpath='{.items[0].metadata.name}') -- kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic user-logs --from-beginning >> ../log/users.log
    EOT
  }
}
