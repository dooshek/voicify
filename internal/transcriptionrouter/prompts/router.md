<objective>
Your task is to analyze spoken text transcription and determine which action should be performed based on the user's intention.
</objective>

<rules>
- Commands should always be spoken at the beginning of the transcription.
- The user can choose one of the possible_actions. Each action includes examples of possible commands that the user may speak, but these can also be variations of words or sentences.
- The user can also choose not to perform any action.
- If no clear action is detected, use "no_action" as the action value.
- If multiple actions could match, choose the one with the highest confidence.
- For very short or empty transcriptions, set action to "no_action" and explain in thoughts.
- The "transcription_without_command" field should contain only the part of the transcription that follows the command, or the original transcription if no command was detected.
</rules>

<possible_actions>
%s
</possible_actions>

<original_transcription>
%s
</original_transcription>

Return a JSON object with the following fields in exactly this format, without any additional characters before `{` and after `}`:

{
  "thoughts": string,
  "action": string,
  "confidence": float,
  "transcription_without_command": string
}

<example>
If the possible actions are:
- "search": "search for", "find", "look up"
- "navigate": "go to", "open", "navigate to"
- "call": "call", "phone", "dial"
- "message": "send message", "text", "message"

And the original transcription is: "go to settings and change my password"

The expected output would be:
{
  "thoughts": "The transcription starts with 'go to' which matches the 'navigate' action pattern.",
  "action": "navigate",
  "confidence": 0.95,
  "transcription_without_command": "settings and change my password"
}

Example 2 - Clear search intent:
If the original transcription is: "find restaurants near me"

The expected output would be:
{
  "thoughts": "The transcription starts with 'find' which is a clear match for the 'search' action.",
  "action": "search",
  "confidence": 0.98,
  "transcription_without_command": "restaurants near me"
}

Example 3 - No clear action:
If the original transcription is: "I'm wondering what time it is"

The expected output would be:
{
  "thoughts": "The transcription doesn't start with any of the command patterns from the possible actions list.",
  "action": "no_action",
  "confidence": 0.85,
  "transcription_without_command": "I'm wondering what time it is"
}

Example 4 - Ambiguous command:
If the original transcription is: "get me directions to the airport"

The expected output would be:
{
  "thoughts": "The phrase 'get me directions to' could be interpreted as either 'navigate' or 'search'. Since it's about directions, 'navigate' seems more appropriate.",
  "action": "navigate",
  "confidence": 0.75,
  "transcription_without_command": "the airport"
}

Example 5 - Very short transcription:
If the original transcription is: "um"

The expected output would be:
{
  "thoughts": "The transcription is too short and doesn't contain any actionable command.",
  "action": "no_action",
  "confidence": 0.99,
  "transcription_without_command": "um"
}
</example>
