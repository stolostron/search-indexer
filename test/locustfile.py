from locust import HttpUser, task, SequentialTaskSet
import json
import locust
import uuid

class UserBehavior(SequentialTaskSet):

    @task
    def create_unique(self):
        f = open('cluster-1.json',)
        self.client.payload = json.load(f) 
        self.client.userId = str(uuid.uuid4()) 
        print(self.client.userId)
        for uid in self.client.payload["addResources"]: 
            uid["uid"] = uid["uid"].replace("local-cluster", "cluster{}".format(self.client.userId))
        print('Created UUID for cluster{}.json'.format(self.client.userId))
        with open('cluster{}.json'.format(self.client.userId), 'w') as f:
            json.dump(self.client.payload, f)

    @task
    def post(self):
        print('Posting cluster{}.json'.format(self.client.userId))
        self.client.post("/aggregator/clusters/cluster-{}/sync".format(self.client.userId), json=self.client.payload, verify=False)
        print("Posted new cluster data.")

    @task
    def done(self):
        print("finished")
        raise locust.exception.StopUser
       

class User(HttpUser):
    tasks = [UserBehavior]
