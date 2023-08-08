Notes for Search POC with Kafka


# Deploy Kafka

Use the following make target to deploy Kafka on an OpenShift cluster.
```
make setup-deploy-kafka
```

## Configure Kafka brokers

To configure the Kafka brokers, run the following make target and paste the generated command.
```
make setup-kafka-brokers
```


## Questions

1. Topics - Is using 1 topic per cluster scalable?
2. Message size - Is sending a message per resource scalable?
3. Initial or full state - How often should collector send the full state. Should collectors read the existing state from kafka when initializing?


## Kafka Events:

Possible list of event types produced by collectors.

* collector_started      - Will start streaming the initial state.
* collector_initialized  - Done streaming initial state, will start producing events for deltas. 
* resource_add           - Example: {uid:123 Properties:["a":"aaa", "b":"bbb", "c":"ccc"]}
* resource_update        - Example: {uid:123 Properties:["b": nil, "c":"changedVal", "d":"newProp"]}
* resource_delete        - Example: {uid:123}