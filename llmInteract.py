from typing import List
import requests
import json
import logging

company_name = "Google"

ollama_prompt_config = { 
    "model": "llama3.1:latest",
    "messages": [],
    "stream": False}


        
def chat(messages: List[dict[str, str]]) -> str: 
    model = "llama3.1:latest" 
    messages.insert(0, {"role": "system", "content": "Messages coming to you will have the format: <user id>:<message>. Your name is BeeBrain and you are slack chat bot. Keep answers as short as possible, friendly. You can reply without saying your name."}) 
    logging.info(f"Invoking LLM chat enpoint ...") 
    r = requests.post(
        "http://0.0.0.0:11434/api/chat",
        json={"model": model, "messages": messages, "stream": True},
	stream=True
    )
    r.raise_for_status()
    output = ""

    for line in r.iter_lines():
        body = json.loads(line)
        if "error" in body:
            raise Exception(body["error"])
        if body.get("done") is False:
            message = body.get("message", "")
            content = message.get("content", "")
            output += content
            # the response streams one token at a time, print that as we receive it

        if body.get("done", False):
            message["content"] = output
            return message["content"]


def main():
    messages = []

    while True:
        user_input = input("Enter a prompt: ")
        if not user_input:
            exit()
        print()
        messages.append({"role": "user", "content": user_input})
        message = chat(messages)
        messages.append(message)
        print("\n\n")

if __name__ == "__main__":
    main()
    