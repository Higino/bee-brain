from slack_sdk import WebClient
from slack_sdk.errors import SlackApiError
from typing import List, Dict
import logging

def get_user_name (client: WebClient, user_id: str) -> str:
    try:
        response = client.users_info(
            user=user_id
        )
        user_info = response.get('user', {})
        real_name = user_info.get('real_name', 'Unknown User')

        return real_name
    except SlackApiError as e:
        assert e.response["error"]
        return 'Unknown User'



def fetch_thread_message(client: WebClient, channel: str, thread_ts:str, max_messages: int = 50) -> List[Dict]:
    try:
        cursor = None
        response = client.conversations_replies(
            channel=channel,
            ts=thread_ts,
            limit=max_messages,
            cursor=cursor
        )
        messages = response.get('messages', [])
        logging.info (f"Thread messages extracted contains {len(messages)} messages")
        #messageList = [{"role": "user", "content": f"{get_user_name(item.get('user', ''))}: {item.get('text', '')}"} for item in messages]
        messageList = [{"role": "user", "content": f"{(item.get('user', ''))}: {item.get('text', '')}"} for item in messages]
        
        return messageList
    except SlackApiError as e:
        assert e.response["error"]
        return []



