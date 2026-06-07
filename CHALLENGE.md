# Welcome!

This challenge is designed to see how effectively you collaborate with AI coding assistants to build in any language. We will evaluate your ability to leverage AI tools, craft effective prompts, and make smart architectural decisions.

# The Scenario

You're building a **Movie Quote Search Engine** for a mental wellness app. Users describe a situation or feeling, and your tool helps find the most relevant movie quotes when they're going through difficult moments. Your tool needs to understand the emotional intent behind messy, real-world queries and surface quotes that genuinely resonate, not just keyword matches. The product manager views this feature as strong potential for increasing user engagement with app.

# Your Mission

Build a Golang CLI tool called quote-finder that:

1. **Reads** a JSON file ([`quotes.json`](quotes.json)) containing:
- A user query describing a situation/emotion
- A list of famous movie quotes with their sources
2. **Ranks** the quotes by relevance to the query using any approach you choose
3. **Outputs** the top 3 most relevant quotes with:
    - The quote text
    - The movie it's from
    - A relevance score (0.0 to 1.0)

## Requirements

- Implement in Golang (we don't expect you to be a Go expert!)
- Use any AI coding assistant (Claude, ChatGPT, Cursor, Copilot, etc.)
- Handle the provided sample data (~20 quotes)
- Include basic error handling
- Keep it simple

## Sample Data

See the [quotes.json](quotes.json) file for sample input data.

## Expected Output

Your program should produce output similar to this:

```
Top 3 quotes for: "I need motivation to keep going when things are tough"

1. [0.92] "Just keep swimming." - Dory (Finding Nemo)
2. [0.87] "Get busy living, or get busy dying." - Andy Dufresne (The Shawshank Redemption)
3. [0.81] "The only way out is through." - John Ottway (The Grey)
```

*Note: Your scores may vary depending on your ranking approach!*

## User Engagement

Now that you've implemented it, what is an additional feature or capability you would add that would help further the business goal of increasing user engagement? How would it work?

## Stretch Goal (Nice-to-Have)

**Support custom queries via command line**: Allow users to override the query in the JSON file by passing a custom query as a CLI argument.

**Usage examples:**

```
# Use query from JSON file (baseline)
go run main.go quotes.json

# Override with custom query (stretch goal)
go run main.go quotes.json --query "I just got rejected and feel like giving up"
```

## Deliverable

Create a `README.md` file that includes:

1. **Approach (2-4 sentences)**. Briefly describe your ranking strategy and implementation approach.
2. **Key Prompts Used**. Copy-paste the most important prompts you used with your AI assistant. Show your iteration if relevant!
3. **Reflections**. What worked well with the AI assistant? What was challenging or required multiple attempts? Any surprises about coding in Go with AI help?
4. **User Engagement**. Add to your `README.md` file your thoughts on one additional feature/capability that would increase user engagement.
5. **Instructions on how to run your application**
6. **Stretch Goal**. Brief note on whether you implemented it, and any interesting decisions or prompts involved.

## Submission

In a public GitHub repository, please submit:

- Your `README.md` file (the main deliverable)
- Your Go source code (e.g., `main.go`, `go.mod`, etc.)
- Any additional files needed to run your solution

And, simply share your GitHub repository URL with us when you are finished.

## Time

Please timebox this challenge to 1 hour. We don’t want you to spend your weekend on this!

**Important**: Prioritize a working core solution over a perfect stretch goal!

---

Good luck, and enjoy the challenge!  
If you have any questions, please don't hesitate to reach out.