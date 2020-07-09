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
   - **Average rSET time:  15.131563470999987**
   - **Average rGET time:  3.9848622660000013**
   
   **EXPERIMENT (2.3)**
   - same as 2.2 but 10000 requests
   - **Average rSET time:  7.416319824999994**
   - **Average tGET time:  11.369112573299988**

   **EXPERIMENT (2.4)**
   - 3 clients launching 1000 concurrent rSET requests each against the same proxy
   - pool of 20 cache nodes
   - value size = 1313 B
   **Average rSET time: 8.89 ms**

   **EXPERIMENT (2.5)**
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
   - **Average mkSET time:  7.292272099999998**
   - **Average mkGET time:  27.343824769999998**

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

### Multi-Proxy Infinicache

   We start our evaluation with the following configuration:
        - 2 proxies, each managing a pool of 20 nodes

   ##### MK OPS
   
  **EXPERIMENT (4.0)**
  - 2000 mkSET of different high-level-keys containing 9
     low-level-keys (1313 B each) followed by 2000 mkGET of 
     3 low-level-keys (1313 B each) by 1 single client
  - **Average mkSET time:  14.603984662000023**
  _ **Average mkGET time:  78.19207285049993**
   
   
   **EXPERIMENT (4.0)**
   - 3000 mkSET of different high-level-key formed by 
   9 low-level-keys followed by 3000 mkGET of 
   1 high-level-key formed by 3 low-level-keys performed 
   by 1 single client
   - low-level-value size = 1313 B 
   Average mkSET time:  23.630190867666695
   Average mkGET time:  44.61256727299992
   
   **EXPERIMENT (4.1)**
   - 10000 mkSET of different high-level-keys containing 9
   low-level-keys followed by 10000 mkGET of 3 low-level-keys 
   by 1 single client
   - low-level-value size = 1313 B each
   - **Average mkSET time:  16.560812107200025**
   - **Average mkSET time:  160 ms**

   ##### R Ops
   
   **EXPERIMENT (5.0)**
   - 3000 consecutive rSET followed by 3000 rGET 1313 B kv pairs
   - **Average mkSET time:  10.533003250333326**
   - **Average mkGET time:  25.381107197999974**
      
   
   **EXPERIMENT (5.1)**
   - 10000 rSET of 1313 B key-values pairs followed by same rGET
   - **Average rSET time:  10.188824880499956**
   - **Average rGET time:  39.50156890679989**
   
   ### 10 nodes 1 proxy
   
   EC - 3000 req, more it crashes
   
   160 B  
   
   300 SET 9 kv pair
   300 GET 3 kv pair
   SET stats 28.710982 114.785311 63.2993797873754 11.607186965186658 [34.594001500000005 34.594001500000005 34.594001500000005 34.594001500000005]
   GET stats  156.70109300000001 1797.3479969999996 611.3910521627907 335.9847512201305 [163.0378225 163.0378225 163.0378225 163.0378225]
 
   3 clients executing each concurrently
   3000 SET 9 kv pair
   3000 GET 3 kv pair
   c1 - SET stats  1.464457 440.436431 7.482554969666652 12.53533603071474 [7.749632999999999 10.326884 13.867417 36.38768399999999]
   c2 - SET stats  1.480451 446.30534500000005 7.493667074000011 12.61736682168206 [7.750923 10.530379 14.139126000000001 35.546514]
   c3 - SET stats  1.492535 459.683672 7.50933457833332 12.77373879547824 [7.7105879999999996 10.290621 13.83002 38.556171]
   c1 - GET stats  28.710276 1972.076161 210.34167797498378 252.984037778915 [212.55034799999999 568.698938 776.6858354999999 1176.258107]
   c2 - GET stats  25.137197999999998 2181.1512249999996 208.80013846595202 261.03164815718884 [200.880827 568.7021769999999 796.670385 1211.2841254999998]
   c3 - GET stats  27.3349998 2122.1535 208.80013846595202 260.000432432 [200.880827 568.7021769999999 796.670385 1211.2841254999998]
   
   1300 B
   
   300 SET 9 kv pairs 
   300 GET 3 kv pairs
   SET stats  23.227061999999997 131.632052 64.5216455747509 16.375477619981393 [27.007247 27.007247 27.007247 27.007247]
   GET stats  136.843314 1247.706197 526.1609912159471 230.10313348648634 [163.16092350000002 163.16092350000002 163.16092350000002 163.16092350000002]
   
   300 SET 1 kv pair
   300 GET 1 kv pair
   SET stats  3.699341 38.608823 6.986428810631229 2.8106319060167015 [3.7318835 3.7318835 3.7318835 3.7318835]
   GET stats  38.921535999999996 1422.0078350000001 186.0718825215946 222.55722944560847 [42.5784685 42.5784685 42.5784685 42.5784685]

   MK 
   
   160 B
   
   SET 300 HLK with 9 LLK
   GET 300 HLK with 3 LLK
   SET stats  1.886626 18.735651 6.33237512666667 2.716169279175395 [2.068223 2.068223 2.068223 2.068223]
   GET stats  5.65168 485.729886 55.95648995666668 44.74148829932032 [7.421599 7.421599 7.421599 7.421599]

   SET 3000 HLK with 9 LLK
   GET 3000 HLK with 3 LLK
   SET stats  1.336293 6237.0218429999995 7.96943970166665 113.7908375836424 [1.639417 1.6935324999999999 1.69669 1.7019695000000001]
   GET stats  26.108932000000003 1297.426499 94.81269793699978 109.74672263163468 [30.768836 31.397760499999997 31.675868 32.027298]

   1300 B
   
   SET 3000 HLK with 9 LLK
   GET 3000 HLK with 3 LLK
   SET stats  1.410617 49.392752 5.681739208333331 3.34020208018547 [1.652797 1.684133 1.694435 1.6983635000000001]
   GET stats  24.446341999999998 1413.5966779999999 92.75126664833309 104.8908930431621 [31.137768 31.528844 31.582381499999997 31.650127499999996]

   R
   
   160 B
   
   3000 rSET, 3000 rGET
   SET stats  0.951957 19.209856000000002 3.5166847486666746 2.1233754218822574 [4.8879529999999995 6.218203 6.957434999999999 10.576148]
   GET stats  2.177642 40.301633 5.0445013563333285 3.0744234520405813 [6.159378 8.817355000000001 10.962438 16.408267000000002]

   1300 B
   
   3000 rSET, 3000 rGET
   SET stats  0.988482 34.704876000000006 4.478564037999996 2.770862824343977 [5.742141 7.395116 9.122527 14.354230000000001]
   GET stats  2.273105 56.193619 7.009821354666675 4.467119877265521 [8.89286 13.009936 15.651531 21.732879999999998]
   
   
   ### 20 nodes 1 proxy
      
  EC - 3000 req
  
  160 B  
  
  
  1300 B
  
  
  3000 SET 1 kv pair
  3000 GET 1 kv pair
  SET stats  22.883977 764.404052 86.52103736221251 34.25369851004978 [98.9168605 127.172989 144.875133 189.1345825]
  GET stats  25.166596 4721.842609 141.33806932155954 467.60814641417767 [68.99743749999999 84.2181975 119.21165349999998 3047.4348760000003]
  
  
  3 clients executing each concurrently
  3000 SET 9 kv pair
  3000 GET 3 kv pair
  c1 - SET stats  1.464457 440.436431 7.482554969666652 12.53533603071474 [7.749632999999999 10.326884 13.867417 36.38768399999999]
  c2 - SET stats  1.480451 446.30534500000005 7.493667074000011 12.61736682168206 [7.750923 10.530379 14.139126000000001 35.546514]
  c3 - SET stats  1.492535 459.683672 7.50933457833332 12.77373879547824 [7.7105879999999996 10.290621 13.83002 38.556171]
  c1 - GET stats  28.710276 1972.076161 210.34167797498378 252.984037778915 [212.55034799999999 568.698938 776.6858354999999 1176.258107]
  c2 - GET stats  25.137197999999998 2181.1512249999996 208.80013846595202 261.03164815718884 [200.880827 568.7021769999999 796.670385 1211.2841254999998]
  c3 - GET stats  27.3349998 2122.1535 208.80013846595202 260.000432432 [200.880827 568.7021769999999 796.670385 1211.2841254999998]
  
  1300 B
  
  300 SET 9 kv pairs 
  300 GET 3 kv pairs
  SET stats  23.227061999999997 131.632052 64.5216455747509 16.375477619981393 [27.007247 27.007247 27.007247 27.007247]
  GET stats  136.843314 1247.706197 526.1609912159471 230.10313348648634 [163.16092350000002 163.16092350000002 163.16092350000002 163.16092350000002]
  
  300 SET 1 kv pair
  300 GET 1 kv pair
  SET stats  3.699341 38.608823 6.986428810631229 2.8106319060167015 [3.7318835 3.7318835 3.7318835 3.7318835]
  GET stats  38.921535999999996 1422.0078350000001 186.0718825215946 222.55722944560847 [42.5784685 42.5784685 42.5784685 42.5784685]

  MK 
  
  160 B
  
  SET 300 HLK with 9 LLK
  GET 300 HLK with 3 LLK
  SET stats  1.886626 18.735651 6.33237512666667 2.716169279175395 [2.068223 2.068223 2.068223 2.068223]
  GET stats  5.65168 485.729886 55.95648995666668 44.74148829932032 [7.421599 7.421599 7.421599 7.421599]

  SET 3000 HLK with 9 LLK
  GET 3000 HLK with 3 LLK
  SET stats  1.336293 6237.0218429999995 7.96943970166665 113.7908375836424 [1.639417 1.6935324999999999 1.69669 1.7019695000000001]
  GET stats  26.108932000000003 1297.426499 94.81269793699978 109.74672263163468 [30.768836 31.397760499999997 31.675868 32.027298]

  1300 B
  
  SET 3000 HLK with 9 LLK
  GET 3000 HLK with 3 LLK
  SET stats  1.410617 49.392752 5.681739208333331 3.34020208018547 [1.652797 1.684133 1.694435 1.6983635000000001]
  GET stats  24.446341999999998 1413.5966779999999 92.75126664833309 104.8908930431621 [31.137768 31.528844 31.582381499999997 31.650127499999996]

  R
  
  160 B
  
  3000 rSET, 3000 rGET
  SET stats  0.951957 19.209856000000002 3.5166847486666746 2.1233754218822574 [4.8879529999999995 6.218203 6.957434999999999 10.576148]
  GET stats  2.177642 40.301633 5.0445013563333285 3.0744234520405813 [6.159378 8.817355000000001 10.962438 16.408267000000002]

  1300 B
  
  3000 rSET, 3000 rGET
  SET stats  0.988482 34.704876000000006 4.478564037999996 2.770862824343977 [5.742141 7.395116 9.122527 14.354230000000001]
  GET stats  2.273105 56.193619 7.009821354666675 4.467119877265521 [8.89286 13.009936 15.651531 21.732879999999998]
   
   
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