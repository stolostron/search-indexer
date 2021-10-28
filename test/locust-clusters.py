from locust import HttpUser, task, between, TaskSet
import json
import uuid
import urllib3
urllib3.disable_warnings()

clusterCount = 0 # Used to name clusters sequentially.

class ClusterBehavior(TaskSet):
# A cluster sends 2 types of messages:
#  1. Full state sync. This typically happens with the first request.
#  2. Update state.

    def send_full_state_payload(self):
        f = open("cluster-template.json",)
        j = json.load(f)        
        for resource in j["addResources"]: 
            resource["uid"] = resource["uid"].replace("local-cluster",self.user.name)
        self.client.payload = j
        self.do_post()
        print("%s - sent full state" % self.user.name)

    def send_update_payload(self):
        f = open("cluster-update-template.json",)
        j = json.load(f)        
        for resource in j["addResources"]: 
            resource["uid"] = "{}/{}".format(self.user.name, str(uuid.uuid4()) )
            resource["properties"]["name"] = "gen-name-{}".format(str(uuid.uuid4()) )
        self.client.payload = j
        self.do_post()
        print("%s - sent update" % self.user.name)

    def do_post(self):
        self.client.post("/aggregator/clusters/{}/sync".format(self.user.name), json=self.client.payload, verify=False)

    def on_start(self):
        self.send_full_state_payload()

    @task
    def send_update(self):
        self.send_update_payload()
       
# A Cluster is equivalent to a User.
class Cluster(HttpUser):
    name = ""
    tasks = [ClusterBehavior]
    wait_time = between(5, 20)

    def on_start(self):
        global clusterCount
        self.name = "cluster{}".format(clusterCount)
        clusterCount = clusterCount + 1
        print("Starting cluster [%s]" % self.name)