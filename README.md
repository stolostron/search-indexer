# search-indexer
Index resources for search.


## Development

1. Setup development `make setup`
2. Run tests `make tests`
3. Run locally `make run`

Explore other supported tasks with `make help`.

## Unit Test

Unit tests mock the pgx connection object. More info: https://github.com/driftprogramming/pgxpoolmock


## Scale Test

Prerequisites: 

You must have **python**, **pip** and **locust** installed

*  Download latest version of python: https://www.python.org/downloads/
*  Install pip: https://pip.pypa.io/en/stable/installation/
*  Install locust  `pip install locust`
 
### Running Locust

1. Once we have the search indexer running with steps above, navigate to the **test** folder and run the following command:
`locust -f locust-clusters.py`

2. Follow the url provided (http://localhost:8089) and input load parameters in ui prompt.
Alternatively, we can just use this one line command instead of the UI for more automatated flow:
`locust -f locust-clusters.py --headless --users 10 --spawn-rate 5 -H https://localhost:3010`

For more on locust visit https://docs.locust.io/en/stable/index.html

### Running Locust inside your cluster

1. Build the docker image.
    ```
    docker build -f Dockerfile.locust .
    ```
2. Publish the docker image.
3. Deploy the following job on your cluster
    ```
    oc apply -f test/locustJob.yaml
    ```

Rebuild Date: 2025-02-25
