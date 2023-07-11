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
        with open("cluster-templates/sno-100k.json", "r") as template_file:
            template_string = template_file.read().replace("<<CLUSTER_NAME>>", self.user.name)
        f = io.StringIO(template_string)
        j = json.load(f)        
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
        resp = self.client.post("/aggregator/clusters/{}/sync".format(self.user.name), json=self.client.payload, verify=False)
        print("[%s] response code: %s" % (self.user.name, resp.status_code))
        if resp.status_code != 200: # 429
            self.user.retries = self.user.retries + 1
            print("[%s] Indexer was busy. Waiting %d seconds and retrying." % (self.user.name, self.user.retries * 2))
            time.sleep(self.user.retries * 2)
            self.do_post()
        else:
            print("[%s] Completed do_post() with %s retries." % (self.user.name, self.user.retries))
            self.user.retries = 0


    def on_start(self):
        self.send_full_state_payload()
        time.sleep(60)

    @task
    def send_update(self):
        # self.send_update_payload()
        self.send_full_state_payload()
       
# A Cluster is equivalent to a User.
class Cluster(HttpUser):
    name = ""
    tasks = [ClusterBehavior]
    wait_time = between(30, 60)
    retries = 0

    def on_start(self):
        global clusterCount
        self.name = "locust-{}".format(clusterCount)
        clusterCount = clusterCount + 1
        print("Starting cluster [%s]" % self.name)