import os
import logging
from slack_sdk import WebClient
from slack_sdk.errors import SlackApiError
from slack_bolt.adapter.flask import SlackRequestHandler
from slack_bolt import App
from dotenv import load_dotenv, find_dotenv
from flask import Flask, request, jsonify
from llmInteract import chat
import slack_utilities as utils

logging.basicConfig(level=logging.INFO)
load_dotenv(find_dotenv())
SLACK_BOT_TOKEN = os.environ.get("SLACK_BOT_OAUTH_TOKEN")
SLACK_SIGNING_SECRET = os.environ.get("SLACK_SIGNING_SECRET")
SLACK_BOT_USER= os.environ.get("SLACK_BOT_USER")

app = App(token=SLACK_BOT_TOKEN)

flask_app = Flask(__name__)
handler = SlackRequestHandler(app)
slack_client = WebClient(token=SLACK_BOT_TOKEN)

def add_reaction(channel, ts, reaction):
    try:
        response = slack_client.reactions_add(
            channel=channel,
            timestamp=ts,
            name=reaction
        )
        logging.info(f"Reaction {reaction} added to message {ts} in {channel}")
        return response
    except SlackApiError as e:
        assert e.response["error"]

def remove_reaction(channel, ts, reaction):
    try:
        response = slack_client.reactions_remove(
            channel=channel,
            timestamp=ts,
            name=reaction
        )
        logging.info(f"Reaction {reaction} removed from message {ts} in {channel}")
        return response
    except SlackApiError as e:
        assert e.response["error"]

@app.event("app_mention")
def handle_mentions(body, say):
    logging.info(f"Received app mention: {body}")
    mention = f"<@{SLACK_BOT_USER}>"
    text = body.get("text", "")
    event = body.get("event", {})
    channel = event.get("channel", "") 
    timestamp = event.get("ts", "")
    emoji = "eyes"
    add_reaction(channel, timestamp, emoji)

    text = text.replace(mention, "").strip()
    response = "Yoo"

    slack_client.chat_postMessage(
            channel=channel,
            text=response,
            thread_ts=None
        )
    remove_reaction(channel, timestamp, emoji)



@app.message(".*")
def handle_messages(body, say):
    event = body.get("event", {})
    text = body.get("text", "")
    channel = event.get("channel", "")

    
    if( 'thread_ts' in event):
        parent_ts = event.get("thread_ts", "")
        timestamp = event.get("ts", "")
        emoji = "eyes"
        history = utils.fetch_thread_message(slack_client, channel, parent_ts)
        
        response = chat(history)

        logging.info(f"Response from LLM: {response}")

        slack_client.chat_postMessage(
            channel=channel,
            text=response,
            thread_ts=parent_ts
        )
        remove_reaction(channel, timestamp, emoji)

@flask_app.route("/", methods=["POST"])
def index():
    data = request.json
    # When we activate slack events on our slack app, we will receive a challenge and act accordingly (https://api.slack.com/apis/events-api#challenge)
    if( data.get("challenge", False)):
        return jsonify({"challenge": data.get("challenge")})

    response = handler.handle(request)
    return response

@flask_app.route("/events", methods=["POST"])
def slack_events():
    data = request.json

    if not data:
        return jsonify({"error": "No data received"}), 400
    
    return handler.handle(request)

@flask_app.route("/", methods=["GET"])
def slack():
    return jsonify({"message": "Hello World!"})

if __name__ == "__main__":
    flask_app.run(port=3000)