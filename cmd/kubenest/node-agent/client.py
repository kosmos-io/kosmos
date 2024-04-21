import websocket
import time
import urllib
import os
import ssl
import base64

# test username and password
username = os.environ.get('WEB_USER', '')
password = os.environ.get('WEB_PASS', '')

# combine the USE and PASS
credentials = f"{username}:{password}"
auth_header = base64.b64encode(credentials.encode('utf-8')).decode('utf-8')

# combine Authorization
headers = {"Authorization": f"Basic {auth_header}"}
current_script_path = os.path.abspath(__file__)
current_directory = os.path.dirname(current_script_path)


# action for message
def on_message(ws, message):
    print(message)
# action for error
def on_error(ws, error):
    print(f"connection error {error}")
# action for close
def on_close(ws, close_status_code, close_msg):
    print(f"### closed ### close_status_code: {close_status_code}, close_msg:{close_msg}")
# action on open
def on_open(ws):
    script = '''
import time
for x in range(100):
    print(f"x = {x}")
    time.sleep(0.25)
'''
    print("sending data...")
    ws.send(script,websocket.ABNF.OPCODE_BINARY)
    print("done.")
# action for script execute
def on_script(file_path):
    def on_open(ws):
        try:
            with open(file_path, "rb") as file:
                file_data = file.read()
                print("Sending data...")
                ws.send(file_data,websocket.ABNF.OPCODE_BINARY)
                print("Done.")
                ws.send('EOF'.encode('utf-8'), websocket.ABNF.OPCODE_BINARY)
        except FileNotFoundError:
            print(f"File '{file_path}' not found.")
    return on_open
# action for upload file
def on_upload(file_path):
    def on_open(ws):
        try:
            with open(file_path, "rb") as file:
                file_data = file.read()
                print("Sending data...")
                ws.send(file_data,websocket.ABNF.OPCODE_BINARY)
                print("Done.")
                ws.send('EOF'.encode('utf-8'), websocket.ABNF.OPCODE_BINARY)
        except FileNotFoundError:
            print(f"File '{file_path}' not found.")
    return on_open

def cmd_test():
#     websocket.enableTrace(True)
    params = urllib.parse.quote("ls -l", encoding='utf-8')
    ws = websocket.WebSocketApp(f'wss://127.0.0.1:5678/cmd/?command={params}',
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close,
                                header=headers)
    ws.run_forever(sslopt={"cert_reqs": ssl.CERT_NONE})
def py_script_test():
    print("python script test")
#     websocket.enableTrace(True)
    ws = websocket.WebSocketApp(f'wss://127.0.0.1:5678/py/?args=10&&args=10',
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close,
                                header=headers)
    on_open = on_script(os.path.join(current_directory,"count.py"))
    ws.on_open = on_open
    ws.run_forever(sslopt={"cert_reqs": ssl.CERT_NONE})
def shell_script_test():
    print("shell script test")
    ws = websocket.WebSocketApp(f'wss://127.0.0.1:5678/sh/?args=1&&args=10',
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close,
                                header=headers)
    on_open = on_script(os.path.join(current_directory,"count.sh"))
    ws.on_open = on_open
    ws.run_forever(sslopt={"cert_reqs": ssl.CERT_NONE})
def upload_test():
    print("upload file test")
    file_name = urllib.parse.quote("deploy.yaml", encoding='utf-8')
    file_path = urllib.parse.quote("/tmp/websocket", encoding='utf-8')

    ws = websocket.WebSocketApp(f'wss://127.0.0.1:5678/upload/?file_name={file_name}&&file_path={file_path}',
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close,
                                header=headers)
    on_open = on_upload(os.path.join(current_directory,"app.py"))
    ws.on_open = on_open
    ws.run_forever(sslopt={"cert_reqs": ssl.CERT_NONE})
if __name__ == "__main__":
    cmd_test()
    # upload test
    upload_test()
    # test shell script
    shell_script_test()
    # test python script
    py_script_test()

