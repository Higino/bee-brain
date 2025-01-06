import os

from slack_sdk import WebClient
from slack_sdk.errors import SlackApiError
from slack_bolt.adapter.flask import SlackRequestHandler
from slack_bolt import App
from dotenv import load_dotenv, find_dotenv
from flask import Flask, request, jsonify

load_dotenv(find_dotenv())
SLACK_BOT_TOKEN = os.environ.get("SLACK_BOT_OAUTH_TOKEN")
SLACK_SIGNING_SECRET = os.environ.get("SLACK_SIGNING_SECRET")
SLACK_BOT_USER= os.environ.get("SLACK_BOT_USER")

app = App(token=SLACK_BOT_TOKEN)

flask_app = Flask(__name__)
handler = SlackRequestHandler(app)
slack_client = WebClient(token=SLACK_BOT_TOKEN)

def getBotUserId():
    try:
        response = slack_client.auth_test()
        return response['user_id']
    except SlackApiError as e:
        assert e.response["error"]


if __name__ == "__main__":
    userId = getBotUserId()
    print(f"Bot User ID: {userId}")
    
    
    flask_app.run(port=3000)