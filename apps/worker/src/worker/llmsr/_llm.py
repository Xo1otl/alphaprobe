from dataclasses import dataclass, field

from google import genai
from google.genai.types import GenerateContentConfig, ThinkingConfig

from worker import propose


@dataclass
class LLM(propose.LLM):
    client: genai.Client = field(default_factory=genai.Client)
    model_name: str = "gemini-2.5-flash"

    def generate(self, prompt: str) -> str:
        # Field 2: Use gemini client to generate response
        response = self.client.models.generate_content(
            model=self.model_name,
            contents=prompt,
            config=GenerateContentConfig(thinking_config=ThinkingConfig(thinking_budget=0)),
        )
        if response.text is None:
            msg = "Response text is None"
            raise ValueError(msg)
        return response.text
