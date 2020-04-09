#!/bin/bash
for i in {6378..7380}
do
   echo -e "- protocol: TCP\n  port: ${i}\n  targetPort: ${i}\n  name: port-${i}"
   # echo -e "- containerPort: ${i}\n  name: port-${i}"
done
