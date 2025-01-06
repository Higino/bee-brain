import functools
import inspect
import json
from typing import get_type_hints

def aitool(description: str):
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            return func(*args, **kwargs)
        
        wrapper._aitool_description = description
        return wrapper
    return decorator

def generate_json_description(func):
    if not hasattr(func, '_aitool_description'):
        raise ValueError(f"Function {func.__name__} is not tagged with @AITool")

    signature = inspect.signature(func)
    type_hints = get_type_hints(func)

    params = []
    for name, param in signature.parameters.items():
        param_info = {
            "name": name,
            "type": str(type_hints.get(name, "Any"))
        }
        params.append(param_info)

    return_type = str(type_hints.get('return', 'Any'))

    description = {
        "type": "function",
        "function": {
            "name": func.__name__,
            "description": func._aitool_description,
            "parameters": {
                "type": "object",
                "properties": {param["name"]: param["type"] for param in params}
            },
            "input_params": params,
            "output_type": return_type
        }
    }

    return json.dumps(description, indent=2)

# Example usage
@aitool("Adds two numbers and returns the result")
def add(a: int, b: int) -> int:
    return a + b

@aitool("Greets a person with an optional custom greeting")
def greet(name: str, greeting: str = "Hello") -> str:
    return f"{greeting}, {name}!"

# Generate and print JSON descriptions
print(generate_json_description(add))
print("\n" + "="*50 + "\n")
print(generate_json_description(greet))