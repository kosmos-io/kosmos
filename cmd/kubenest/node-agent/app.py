import asyncio
import websockets
import subprocess
import urllib
import os
from datetime import datetime
import tempfile
import ssl
from OpenSSL import crypto
from functools import wraps
import hashlib
import base64
import hmac
import logging
from urllib.parse import urlparse, parse_qs
import argparse

# logging init
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)
handler = logging.FileHandler(filename='app.log', mode='a')  # w for truncate file and a for append file

# set log formatter
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
handler.setFormatter(formatter)

# addHandler for logger
logger.addHandler(handler)
# embed the username and password if can not get from environment
USER = os.environ.get('WEB_USER', '')
PASSWORD_HASH = hashlib.sha256(os.environ.get('WEB_PASS', '').encode()).hexdigest()

# descriptors for websocket handler method
def authenticate(func):
    @wraps(func)
    async def wrapper(websocket, path):
        # get Authorization from header
        auth_header = websocket.request_headers.get("Authorization")
        if not auth_header:
            logger.info("Unauthorized: Missing Authorization Header")
            await websocket.close(code=1010, reason="Unauthorized: Missing Authorization Header")
            return

        # parse Authorization header info use 1010 for 401 unauth
        try:
            scheme, credentials = auth_header.split()
            if scheme.lower() != "basic":
                logger.info("Unauthorized: Invalid authentication scheme")
                await websocket.close(code=1010, reason="Unauthorized: Invalid authentication scheme")
                return

            decoded_credentials = base64.b64decode(credentials).decode('utf-8')
            username, password = decoded_credentials.split(':', 1)

            # check the username and password
            if username != USER or not hmac.compare_digest(PASSWORD_HASH, hashlib.sha256(password.encode()).hexdigest()):
                logger.info("Unauthorized: Incorrect username or password")
                await websocket.close(code=1010, reason="Unauthorized: Incorrect username or password")
                return
        # throw exception if error occurs
        except Exception as e:
            logger.info(f"Unauthorized: Error processing credentials {e}")
            # use 1011 for server error
            await websocket.close(code=1011, reason="Unauthorized: Error processing credentials")
            return

        # auth pass and execute the native method
        await func(websocket, path)
    # return the wrapper method
    return wrapper



# create self signed cert
def create_self_signed_cert():
    pkey = crypto.PKey()
    pkey.generate_key(crypto.TYPE_RSA, 2048)

    cert = crypto.X509()
    cert.get_subject().C = "CN"  # country
    cert.get_subject().ST = "JiangShu" # state
    cert.get_subject().L = "ShuZhou" # city
    cert.get_subject().O = "Kosmos" # object
    cert.get_subject().OU = "Kosmos" # unit
    cert.get_subject().CN = "kosmos.io"  # domain

    cert.set_serial_number(1000)
    cert.gmtime_adj_notBefore(0)
    cert.gmtime_adj_notAfter(10*365*24*60*60)  # 10 years
    cert.set_issuer(cert.get_subject())  # self signed cert
    cert.set_pubkey(pkey)
    cert.sign(pkey, 'sha256')

    with open('key.pem', 'ab') as f:
        f.write(crypto.dump_privatekey(crypto.FILETYPE_PEM, pkey))
    with open('cert.pem', 'ab') as f:
        f.write(crypto.dump_certificate(crypto.FILETYPE_PEM, cert))

# all handler path entrypoint
@authenticate
async def handler(websocket, path):
    logger.info(f"path = {path}")
    # parse path query params
    url_components = urlparse(path)
    query_params = parse_qs(url_components.query)
    if path.startswith("/upload"):
        # get file_name and file_path
        file_name = query_params.get('file_name', [None])[0]
        file_path = query_params.get('file_path', [None])[0]
        logger.info(f"get file_name:{file_name} and file_path:{file_path}")
        if file_name and file_path:
            await handle_upload(websocket, file_name, file_path)
        else:
            await websocket.send("Invalid file_name or file_path")
    elif path.startswith("/cmd"):
        # Extract command from the path
        command = query_params.get('command', [None])[0]
        if command:
            await handle_cmd(websocket, command)
        else:
            await websocket.send("No command specified")
    elif path.startswith("/py"):
        # Extract args from the path
        args = query_params.get('args', [None])
        await handle_py_script(websocket, args)
    elif path.startswith("/sh"):
        # Extract args from the path
        args = query_params.get('args', [None])
        logger.info(f"get args from path:{args}")
        await handle_shell_script(websocket, args)

    else:
        await websocket.send("Invalid path")

# execute python script
async def handle_py_script(websocket,args):
    return_code = -1
    with tempfile.NamedTemporaryFile(delete=True) as temp:
        # get the file path and file name
        file_path = temp.name
        logger.info(file_path)
        while True:
            try:
                data = await websocket.recv()
                if data.decode('utf-8', 'ignore') == 'EOF':
                    logger.info("finish read data from websocket")
                    break
                temp.write(data)
            except websocket.ConnectionClosed:
                return_code = 1
                await websocket.close(code=1000, reason=f"{return_code}")
                break

        temp.flush()  # flush data to disk
        # combine the shell script command
        command = ['python3','-u',file_path] + args
        logger.info(f"execute python script command:{command}")
        with subprocess.Popen(command,
                              stdout=subprocess.PIPE,stderr=subprocess.STDOUT,
                              bufsize=1,
                              universal_newlines=True) as process:
            for line in process.stdout:
                line = line.rstrip()
                logger.info(f"line = {line}")
                await websocket.send(line)
            # get the process return code
            return_code = process.wait()
            logger.info(f"Command executed with return code: {return_code}")
    await websocket.close(code=1000, reason=f"{return_code}")

# execute shell script
async def handle_shell_script(websocket, args):
    return_code = -1

    # create temp file and delete the file outside the with seq
    with tempfile.NamedTemporaryFile(delete=True) as temp:
        # get file name
        file_path = temp.name
        logger.info(file_path)
        # add execute mod
        while True:
            try:
                data = await websocket.recv()
                if data.decode('utf-8', 'ignore') == 'EOF':
                    logger.info("finish read data from websocket")
                    break
                temp.write(data)
            except websocket.ConnectionClosed:
                return_code = 1
                await websocket.close(code=1000, reason=f"{return_code}")
                break

        temp.flush()  # flush data to disk
        os.chmod(temp.name, os.stat(temp.name).st_mode | 0o111)
        # combine the shell script command
        command = [file_path] + args
        logger.info(f"execute shell script command:{command}")
        with subprocess.Popen(command,
                              stdout=subprocess.PIPE,stderr=subprocess.STDOUT,
                              bufsize=1,
                              universal_newlines=True) as process:
            for line in process.stdout:
                line = line.rstrip()
                logger.info(f"line = {line}")
                await websocket.send(line)
            # get process return_code
            return_code = process.wait()
            logger.info(f"Command executed with return code: {return_code}")
    await websocket.close(code=1000, reason=f"{return_code}")

# execute shell command
async def handle_cmd(websocket, command):
    with subprocess.Popen(command, shell=True,
                          stdout=subprocess.PIPE,stderr=subprocess.STDOUT,
                          bufsize=1,
                          universal_newlines=True) as process:
        for line in process.stdout:
            line = line.rstrip()
            logger.info(f"line = {line}")
            await websocket.send(line)
        # get return_code
        return_code = process.wait()
        logger.info(f"Command executed with return code: {return_code}")
        await websocket.close(code=1000, reason=f"{return_code}")

# upload file to node path and rename the exist file with timestamp and bak str
async def handle_upload(websocket, file_name, directory):
    # Check if the directory exists, if not, create it
    os.makedirs(directory, exist_ok=True)
    file_path = os.path.join(directory, file_name)
    # Check if the file already exists
    if os.path.exists(file_path):
        # Rename the existing file
        timestamp = datetime.now().strftime("%Y-%m-%d-%H%M%S%f")
        bak_file_path = f"{file_path}_{timestamp}_bak"
        os.rename(file_path, bak_file_path)
    return_code=0
    # Receive and write the uploaded file
    # write in binary
    with open(file_path, 'ab') as file:
        while True:
            try:
                data = await websocket.recv()
                if data.decode('utf-8', 'ignore') == 'EOF':
                    logger.info("finish read data from websocket")
                    break
                file.write(data)
            except websockets.ConnectionClosed:
                return_code = 1
                await websocket.close(code=1000, reason=f"{return_code}")
                break
    await websocket.close(code=1000, reason=f"{return_code}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='K8S in K8S node agent server')
    parser.add_argument('--port', metavar='N', type=int, default=5678, help='websocket service port')
    parser.add_argument('--host', metavar='HOST', type=str, default='0.0.0.0', help='websocket service listen address')
    parser.add_argument('--cert', metavar='CERT', type=str, default='cert.pem', help='SSL certificate file')
    parser.add_argument('--key', metavar='KEY', type=str, default='key.pem', help='SSL key file')
    parser.add_argument('--user', metavar='USER',required=True, type=str, default='', help='Username for authentication')
    parser.add_argument('--password', metavar='PASSWORD', required=True, type=str, default='', help='Password for authentication')
    args = parser.parse_args()
    USER = args.user
    PASSWORD_HASH = hashlib.sha256(args.password.encode()).hexdigest()
    if not os.path.exists(args.cert) or not os.path.exists(args.key):
        create_self_signed_cert()
    # add ssl_context for server
    ssl_context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ssl_context.load_cert_chain(certfile=args.cert, keyfile=args.key)
    # start server listen on 0.0.0.0 5678
    start_server = websockets.serve(handler, args.host, args.port, ssl=ssl_context)
    asyncio.get_event_loop().run_until_complete(start_server)
    asyncio.get_event_loop().run_forever()