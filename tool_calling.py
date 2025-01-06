from openai import OpenAI
import instructor
import yfinance as yf
from pydantic import BaseModel, Field

class StockInfo(BaseModel):
    company: str = Field(description="Name of the company")
    ticker: str = Field(description="Ticker symbol of the company")


class add(BaseModel):
    """Add two integers."""

    a: int = Field(..., description="First integer")
    b: int = Field(..., description="Second integer")


class multiply(BaseModel):
    """Multiply two integers."""

    a: int = Field(..., description="First integer")
    b: int = Field(..., description="Second integer")


# The function name, type hints, and docstring are all part of the tool
# schema that's passed to the model. Defining good, descriptive schemas
# is an extension of prompt engineering and is an important part of
# getting models to perform well.
def add(a: int, b: int) -> int:
    """Add two integers.

    Args:
        a: First integer
        b: Second integer
    """
    return a + b


def multiply(a: int, b: int) -> int:
    """Multiply two integers.

    Args:
        a: First integer
        b: Second integer
    """
    return a * b

# enables reposnse model in create call
def getStockPrices():
    company_name= "Google"
    client = instructor.patch(
        OpenAI(
            base_url="http://localhost:11434/v1",
            api_key="ollama",
    ),
    mode=instructor.Mode.JSON)

    resp = client.chat.completions.create(
        model="llama3.1:latest",
        messages=[
        {
            "role": "user",
            "content": f"Return the company name and ticker symbol for {company_name}"
        },
    ],
    response_model=StockInfo,
    )

    stock = yf.Ticker(resp.ticker)
    hist = stock.history(period="1d")
    stock_price = hist["Close"].iloc[-1] if hist["Close"].iloc[-1] else 0
    message = f"The current stock price of {resp.company} is {stock_price}"
    return message

