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
        print("%s - sending full state" % self.user.name)
        with open("cluster-templates/{}".format(self.user.template), "r") as template_file:
            template_string = template_file.read().replace("<<CLUSTER_NAME>>", self.user.name)
        f = io.StringIO(template_string)
        j = json.load(f)        
        self.client.payload = j
        self.do_post()

    def send_update_payload(self):
        print("%s - sending update" % self.user.name)
        f = open("cluster-templates/add-resources.json",)
        j = json.load(f)        
        for resource in j["addResources"]: 
            resource["uid"] = "{}/{}".format(self.user.name, str(uuid.uuid4()) )
            resource["properties"]["name"] = "gen-name-{}".format(str(uuid.uuid4()) )
        self.client.payload = j
        self.do_post()

    def do_post(self):
        resp = self.client.post("/aggregator/clusters/{}/sync".format(self.user.name), json=self.client.payload, verify=False)
        print("[%s] response code: %s" % (self.user.name, resp.status_code))
        if resp.status_code != 200: # The first request is receiving 0 instead of 429.
            self.user.retries = self.user.retries + 1
            print("\t> %s Indexer was busy. Waiting %d seconds and retrying." % (self.user.name, self.user.retries * 2))
            time.sleep(self.user.retries * 2)
            self.do_post()
        else:
            print("%s - completed do_post() with %s retries." % (self.user.name, self.user.retries))
            self.user.retries = 0


    def on_start(self):
        self.send_full_state_payload()
        time.sleep(10)

    @task(10)
    def send_update(self):
        self.send_update_payload()

    @task(1)
    def send_resync(self):
        self.send_full_state_payload()
       
# A Cluster is equivalent to a User.
class Cluster(HttpUser):
    name = ""
    tasks = [ClusterBehavior]
    wait_time = between(5, 120)
    retries = 0
    template = "sno-150k.json" # sno-100k.json, sno-150k.json

    def on_start(self):
        global clusterCount
        self.name = "locust-{}".format(clusterCount)
        clusterCount = clusterCount + 1
        print("Starting cluster [%s]" % self.name)