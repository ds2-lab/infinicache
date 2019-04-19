import boto3
import botocore
import time

start = time.time()
BUCKET_NAME = 'ao.webapp' # replace with your bucket name
KEY = '200mb.csv' # replace with your object key

s3 = boto3.resource('s3')

try:
    s3.Bucket(BUCKET_NAME).download_file(KEY, 'my_local_image.csv')
except botocore.exceptions.ClientError as e:
    if e.response['Error']['Code'] == "404":
        print("The object does not exist.")
    else:
        raise

end = time.time()
print (end - start)