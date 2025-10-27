from locust import HttpUser, task, between, TaskSet
import time
import json
import uuid
import urllib3
import io
urllib3.disable_warnings() # Suppress warning from unverified TLS connection (verify=false)

clusterCount = 0 # Used to name clusters sequentially.

class ClusterBehavior(TaskSet):
# A cluster sends 2 types of messages:
#  1. Full state sync. This typically happens with the first request.
#  2. Update state.

    def send_full_state_payload(self):
        with open("cluster-templates/{}".format(self.user.template), "r") as template_file:
            template_string = template_file.read().replace("<<CLUSTER_NAME>>", self.user.name)
        f = io.StringIO(template_string)
        j = json.load(f)        
        self.client.payload = j
        self.do_post()

    def send_update_payload(self):
        print("%10s - Sending update" % self.user.name)
        f = open("cluster-templates/add-resources.json",)
        j = json.load(f)        
        for resource in j["addResources"]: 
            resource["uid"] = "{}/{}".format(self.user.name, str(uuid.uuid4()) )
            resource["properties"]["name"] = "gen-name-{}".format(str(uuid.uuid4()) )
        self.client.payload = j
        self.do_post()

    def do_post(self):
        resp = self.client.post("/aggregator/clusters/{}/sync".format(self.user.name), name=self.user.template, json=self.client.payload, verify=False)
        if resp.status_code != 200: # The first request is receiving 0 instead of 429.
            self.user.retries = self.user.retries + 1
            print("%10s -\t Received response code %s. Waiting %d seconds and retrying." % (self.user.name, resp.status_code, self.user.retries * 2))
            time.sleep(self.user.retries * 2)
            self.do_post()
        else:
            print("%10s -\t Completed do_post() with %s retries." % (self.user.name, self.user.retries))
            self.user.retries = 0

    def on_start(self):
        self.send_full_state_payload()
        time.sleep(120)

    @task(10)
    def send_update(self):
        self.send_update_payload()

    @task(1)
    def send_resync(self):
        self.send_full_state_payload()


class Cluster(HttpUser):
    abstract = True
    tasks = [ClusterBehavior]
    wait_time = between(5, 300)
    retries = 0
    def on_start(self):
        global clusterCount
        self.name = "locust-{}".format(clusterCount)
        clusterCount = clusterCount + 1
        print("%10s - Starting cluster simulation using template %s" % (self.name, self.template))
        

# Cluster5k - simulates 5k resources.
class Cluster5k(Cluster):
    template = "sno-5k.json"

# Cluster100k - simulates 100k resources.
class Cluster100k(Cluster):
    template = "sno-100k.json"

# Cluster150k - simulates 150k resources.
class Cluster150k(Cluster):
    template = "sno-150k.json"

# ClusterCNV - simulates a CNV cluster.
class ClusterCNV(Cluster):
    template = "vm-cluster.json"