def invoke(count):
    for j in range(count):
        start = time.time()
        fouput = os.popen(
            'curl -s -i http://www.google.com')
        status = fouput.readline()
        print status
        if '200' in status:
            duration = str(time.asctime(time.localtime(time.time())))
            client_latency.append("{0},{1}".format(time.time() - start, duration))

if __name__ == "__main__":
    # handle(
    #     "https://s3.amazonaws.com/demo.cs795.ao/WechatIMG23.jpeg")
    invoke(100)