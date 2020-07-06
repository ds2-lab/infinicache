### Simple Replication Operations

   **EXPERIMENT (2.0) (first test)**
   - 500 Consecutive rSET requests followed by 500 Consecutive rGET requests -
   - value-size = 2 Bytes
   - **Average rSET time:  3.934466188000003 ms**
   - **Average rGET time:  3.7991589939999986 ms**

   **EXPERIMENT (2.1)**
   - 1000 Consecutive rSET requests followed by 1000 Consecutive rGET requests -
   - value-size = 160 B (the same as the average size of Fraudio's data)
   - **Average rSET time:  4.172528283000005 ms**
   - **Average rGET time:  3.4955924879999976 ms**

   **EXPERIMENT (2.2)**
   - same as 2.2 but with value-size = 1313 B
   **Average rSET time:  3.7058410259999963 ms**
   **Average rGET time:  3.4685202250000016 ms**

   **EXPERIMENT (2.3)**
   - 3 clients launching 1000 concurrent rSET requests each against the same proxy
   - pool of 20 cache nodes
   - value size = 1313 B
   **Average rSET time: 8.89 ms**

   **EXPERIMENT (2.4)**
   - 3 clients launching 1000 concurrent rGET requests each against the same proxy
   - pool of 20 cache nodes
   - value size = 1313 B
   **Average rGET time: 2.6 ms**

### Multikey with LowLevelKeys Replication Operations

   **EXPERIMENT (1.0) - (first test)**
   - 500 Consecutive mkSET requests followed by 500 Consecutive mkGET requests
   - low-level-valuessize = 2 B
   - for each SET we set a  high-level-key which contains 9 low-level-keys
   - for each GET we retrieve 3 low-level key-value pairs
   - **Average mkSET time:  4.739296779999997 ms**
   - **Average mkGET time:  10.218054366000006 ms**

   **EXPERIMENT (1.1)**
   - 500 Consecutive mkSET requests followed by 500 Consecutive mkGET requests -
   - low-level-value-size = 160 B (the same as the average size of Fraudio's data)
   - for each SET we set a  high-level-key which contains 9 low-level-keys (1404 B in total)
   - for each GET we retrieve 3 low-level key-value pairs (480 B)
   - **Average mkSET time:  5.7761036919999995 ms**
   - **Average mkGET time:  8.793466639999998 ms**

   **EXPERIMENT (1.2)**
   - 1000 Consecutive mkSET requests followed by 1000 Consecutive rGET requests -
   - low-level-value-size = 160 B (the same as the average size of Fraudio's data)
   - for each SET we set a  high-level-key which contains 9 low-level-keys (1404 B in total)
   - for each GET we retrieve 3 low-level key-value pairs (480 B)
   - **Average mkSET time:  5.987684302000001 ms**
   - **Average mkGET time:  21.932362953000005 ms**

   **EXPERIMENT (1.3)**
   - same as 1.2 but with low-level-value-size = 1313 B which is the max value for Fraudio's Data
   **Average mkSET time:  8.077950724999997 ms**
   **Average mkGET time:  20.197735195999986 ms**

   **EXPERIMENT (1.4)**
   - we launch 3 clients which perform concurrently 1000 mkSET each against the same proxy
   - low-level-value-size = 1313 B
   - we then take the average of the averages of each client as the average time
   - **Average mkSET time:  9.49622103 ms**

   **EXPERIMENT (1.5)**
   - after having set 1000 high-level-keys
   - we launch 3 clients which perform concurrently 1000 mkGET
   of 3 random low-level-keys each against the same proxy
   - low-level-value-size = 1313 B
   - we then take the average of the averages of each client as the average time
   - **Average mkSET time:  50 ms**

### Original Erasure Coding Operations

   **EXPERIMENT (3.1)**
   - 500 Consecutive SET requests followed by 500 Consecutive GET requests -
   - value-size = 160 Bytes
   - **Average SET time:  6.676049121756492 ms**
   - **Average GET time:  159.64132920758456 ms**

   **EXPERIMENT (3.2)**
   - 1000 Consecutive SET requests followed by 1000 Consecutive GET requests -
   - value size = 160 Bytes
   - **Average SET time:  6.410645341658353 ms**
   - **Average GET time:  169.11462692007993 ms**

   **EXPERIMENT (3.3)**
   - tries to simulate the Multikey operation
   - basically we assume that every time we need to save data into the cache
   we need to SET 9 key-value pairs (we replicate the Multi Key operation)
   - then when we need to retrieve data, we need to GET 3 key-value pairs (480 B)
   - **Average SET time:  62.18107363036967 ms**
   - **Average GET time:  500.924516480519 ms**

   **EXPERIMENT (3.4)**
   - 3 clients concurrently SETting and GETting different keys into the system
   - the value of each kv pair is the maximum value from Fraudio's data 1313 B
   - we basically launch 3 concurrent go script which represent the clients and
   then take the average of their averages as the average of the concurrent calls
   - **Average SET time:  105.84354689310699 ms**
   - **Average GET time:  1559.4027978676472 ms**

   OBSERVATION:
   When we perform many operations against infinicache classical version we observe
   that many chunks are lost and then many of them have to be recovered, which obviously
   increases the latency and deteriorates performance. This may happen because of the
   large number of connections that the proxy has to manage and on too many concurrent
   calls many of the packets may get lost and therefore there is the need for recovery

   Next:
   - **DONE** - concurrent with 1 proxy also for R and MK
   - **DONE** - design a testing mechanism which allows to configure the requests rate
        - Conclusion: It doesn't make much sense to set up a request rate as a static parameter for experiments 
        of such dynamic system. It's more worth to make more performance testing where different clients access 
        data simultaneously. This allows seeing how the system reacts in stressful situations and 
        including situations of high request rates.
   - **IN PROGRESS** - make experiments with more proxies, concurrent and consecutive
   - run real workload and analyze logs
   - test other configurations, different number of replicas
   - try simple replication for all data, without low-level-keys,
   but retrieve concurrently multiple keys, based on how many keys
   the Scoring API needs
   - understand how the number of connections is related to economical
   costs in the new setups
   - evaluate to modify infinicache in a way is trully elastic using
   the serverless features of knative; we could basically organize
   the nodes pool of each proxy using consistent hashing and trigger
   a new lambda, whenever the space becomes limitate but the requests
   rate still grows; of course whenever a new node joins a migration
   would be required
e testing by developing some other test cases which can give us a more clear idea about throughput and latency and how they are related each other