import ollama
import json

input1 = 'What is the weather model more appropriante to understand the weather above equator and what is the temperature in for this time of the year?'
input2 = 'What is the details of the customer with id 1234?'
input3 = 'How do we ask a customer about their feelings towardws our product?'
def computeTool(input):
    response = ollama.chat(
        model='llama3.1',
        messages=[{'role': 'System', 'content': 'Please respond with the tools found and only with those ones. If you cannot find a tool respond with "No tool found"'},
                  {'role': 'user', 'content': input}],

        # provide a weather checking tool to the model
        tools=[
        {
            'type': 'function',
            'function': {
                'name': 'get_customer_details_by_id',
                'description': 'Given a customer id gets its billing details.',
                'parameters': {
                    'type': 'object',
                    'properties': {
                        'customer_id': {
                            'type': 'string',
                            'description': 'The id of the customer',
                        },
                    },
                },
            },
        },
        {
            # provide a weather checking tool to the model
            'type': 'function',
            'function': {
                'name': 'get_current_weather',
                'description': 'Get the current weather for a given existing city',
                'parameters': {
                    'type': 'object',
                    'properties': {
                        'city': {
                            'type': 'string',
                            'description': 'The name of the city',
                        },
                    },
                    'required': ['city'],
                },
            },
        },
    ])

    print(response['message']['content'])
    if 'tool_calls' in response['message']:
        print(json.dumps(response['message']['tool_calls']))
    
        # Reflect on the tools output
        reflectPrompt = 'I identified the following tool as being related to the prompt: ' + input + '. The tool found was ' + json.dumps(response['message']['tool_calls']) + \
                        ' please answer with "Tool found" or "No tool fount" and why in less than 50 words?'
        response = ollama.chat(
            model='llama3.1',
            messages=[{'role': 'System', 'content': 'Please respond with yes or no'},
                    {'role': 'user', 'content': reflectPrompt}],
        )
        print(response['message']['content'])


if __name__ == '__main__':
    prompt = ""
    while (prompt != 'exit'):
        prompt = input('Please enter a prompt: ')
        computeTool(input) 