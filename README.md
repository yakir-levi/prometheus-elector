# prometheus-elector

## Acknowledgements

This project is a fork of [prometheus-elector](https://github.com/jlevesy/prometheus-elector) The original project was created by [Jean Levesy](https://github.com/jlevesy). 
All credit for the original work goes to him.

## Changes Made

To ensure that Prometheus Elector works seamlessly in a Prometheus Operator environment,
the following changes have been implemented to enhance functionality:

- Added a new flag `--leader-config`, which specifies the path to the Prometheus leader configuration file.
- Implemented a mechanism to watch for any changes to the leader-config file, as well as the configuration file generated 
by the Prometheus Operator. 



## Configuration Walkthrough: Integrating Prometheus Leader Election Sidecar into Your Deployment

Follow the steps below in order to integrate Prometheus Elector into your Prometheus deployment using the [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) Helm chart. 
The examples provided are based on chart version 51.2.0.

Step 1: Create a Kubernetes Secret for Remote Write Configuration
Begin by creating a Kubernetes Secret that will hold the remote write configuration for the leader.

Refer to the [example/k8s](example/k8s)  folder to find the Kubernetes manifest example and the Prometheus Helm values file, 
which includes all the configurations needed to integrate Prometheus Elector.


```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: prometheus-leader-config-secret
  namespace: infra
type: Opaque
stringData:
  leader.yaml: |-
    remote_write:
      - url: https://your-remote-write-endpoint.com
        remote_timeout: 120s
        name: coralogix
        tls_config:
          insecure_skip_verify: true
        authorization:
          type: Bearer
          credentials: credentials
        follow_redirects: true
        enable_http2: true
        queue_config:
          capacity: 10000
          max_shards: 50
          min_shards: 1
          max_samples_per_send: 2000
          batch_send_deadline: 5s
          min_backoff: 30ms
          max_backoff: 5s
        metadata_config:
          send: true
          send_interval: 1m
          max_samples_per_send: 2000
```

Step 2: Mount the Secret in Prometheus Pods
To mount the Secret in the Prometheus pods, add it to the volumes section under prometheusSpec in the kube-prometheus-stack Helm values file:

```yaml
volumes:
  - name: leader-volume-secret
    secret:
      secretName: prometheus-leader-config-secret
```

Step 3: Create a Role and RoleBinding
Next, create a Kubernetes Role and RoleBinding to grant Prometheus permission to access lease resources. In this setup, the ServiceAccount for Prometheus is prometheus-elector-prometheus.

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prometheus-elector-role
  namespace: infra
rules:
  - apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    verbs:
      - get
      - list
      - watch
      - create
      - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: prometheus-elector-rolebinding
  namespace: infra
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: prometheus-elector-role
subjects:
  - kind: ServiceAccount
    name: prometheus-elector-prometheus
    namespace: infra
  - kind: ServiceAccount
    name: prometheus-operator-prometheus
    namespace: infra
```

Step 4: Configure Additional ServiceMonitor
To scrape metrics from the Prometheus Elector container, add the following configuration under the prometheusSpec section of the Helm values file:

```yaml
additionalServiceMonitors:
  - name: "prometheus-elector"
    endpoints:
      - path: /_elector/metrics
        port: http-elector
    namespaceSelector:
      matchNames:
        - infra
    selector:
      matchLabels:
        app: kube-prometheus-stack-prometheus
        release: prometheus-operator-elector
        self-monitor: "true"
```

Step 5: Add Additional Port for Prometheus Service
To expose the leader elector metrics, configure an additional port under the Prometheus service section of the Helm values file:

```yaml
additionalPorts:
  - name: http-elector
    port: 9095
    targetPort: http-elector
```
Step 6: Add the Prometheus Elector Sidecar Container
Integrate the Prometheus Elector container as a sidecar by adding it under the containers section of the prometheusSpec in the Helm values file:
```yaml
containers:
  - name: prometheus-elector
    image: yakirlevi/prometheus-elector:1.0.0
    imagePullPolicy: Always
    args:
      - -lease-name=prometheus-elector-lease-remote
      - -leader-config=/etc/config_leader/leader.yaml
      - -lease-namespace=infra
      - -config=/etc/prometheus/config_out/prometheus_config.yaml
      - -output=/etc/prometheus/config_out/prometheus.env.yaml
      - -notify-http-url=http://127.0.0.1:9090/-/reload
      - -readiness-http-url=http://127.0.0.1:9090/-/ready
      - -healthcheck-http-url=http://127.0.0.1:9090/-/healthy
      - -api-listen-address=:9095
    command:
      - ./elector-cmd
    ports:
      - name: http-elector
        containerPort: 9095
        protocol: TCP
    securityContext:
      capabilities:
        drop: [ "ALL" ]
      readOnlyRootFilesystem: true
      runAsNonRoot: true
      runAsUser: 1000
    volumeMounts:
      - mountPath: /etc/prometheus/config_out
        name: config-out
      - mountPath: /etc/config_leader
        name: leader-volume-secret
      - mountPath: /etc/pro
```

Step 7: Add Prometheus Elector Init Container
Add the Prometheus Elector as an init container in the Helm values file under the initContainers section of prometheusSpec. The init container is necessary to generate the configuration file for the Prometheus container before it starts. Without this step, Prometheus would fail at startup because it wouldn't be able to find the required configuration file.
```yaml
initContainers:
  - name: init-prometheus-elector
    image: yakirlevi/prometheus-elector:1.0.0
    imagePullPolicy: Always
    args:
      - -config=/etc/prometheus/config_out/prometheus_config.yaml
      - -output=/etc/prometheus/config_out/prometheus.env.yaml
      - -leader-config=/etc/config_leader/leader.yaml
      - -init
    command:
      - ./elector-cmd
    securityContext:
      capabilities:
        drop: [ "ALL" ]
      readOnlyRootFilesystem: true
      runAsNonRoot: true
      runAsUser: 1000
    volumeMounts:
      - mountPath: /etc/prometheus/config_out
        name: config-out
      - mountPath: /etc/config_leader
        name: leader-volume-secret
      - mountPath: /etc/prometheus/config
        name: config
```

Step 8: Override Config Reloader
Due to the limitation in overriding the Prometheus configuration file using a simple flag in the Helm values file, we implemented a solution to modify the output config file parameter utilized by the config-reloader. This enables us to pass the updated configuration to the Prometheus Elector.
To retrieve the current configuration settings for the config-reloader and init-config-reloader in your setup, extract the complete YAML configuration of the StatefulSet (STS) created by the Prometheus Operator. You can do this using the following command:

```base
kubectl get sts <statefulset-name> -n <namespace> -o yaml
```

This command will display the complete configuration of the StatefulSet resource. In the output, look for the init-config-reloader and config-reloader arguments.
Copy the args configuration for init-config-reloader to the initContainers section, and the settings for config-reloader to the containers section.
Here's an example of what the configuration might look like:

```yaml
containers:
  - name: config-reloader
    args:
      - --listen-address=:8080
      - --reload-url=http://127.0.0.1:9090/-/reload
      - --config-file=/etc/prometheus/config/prometheus.yaml.gz
      - --config-envsubst-file=/etc/prometheus/config_out/prometheus_config.yaml
      - --watched-dir=/etc/prometheus/rules/prometheus-prometheus-elector-prometheus-rulefiles-0
      - --log-format=json

initContainers:
  - name: init-config-reloader
    args:
      - --watch-interval=0
      - --listen-address=:8080
      - --config-file=/etc/prometheus/config/prometheus.yaml.gz
      - --config-envsubst-file=/etc/prometheus/config_out/prometheus_config.yaml
      - --watched-dir=/etc/prometheus/rules/prometheus-prometheus-elector-prometheus-rulefiles-0
      - --log-format=json
```

Step 9: Deploy and Verify
At this point, all configurations are ready, and you can deploy the updated Helm values file along with all the Kubernetes manifest resources.
To verify that everything is functioning correctly, connect to your Kubernetes cluster and check that the Prometheus pod includes all the necessary containers, as shown in the example below:

