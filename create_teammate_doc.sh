#!/bin/bash

cat > ./docs/usage-methods/kubiya-cli/teammate-management.md << 'EOL'
# Creating and Managing Teammates with Kubiya CLI

This guide provides detailed instructions for creating, configuring, and interacting with AI teammates using the Kubiya CLI.

## Understanding Teammates

Teammates are AI assistants that help you automate tasks, answer questions, and provide guidance in your specific domain. Each teammate can be customized with different capabilities, knowledge, and access to specific tools.

## Listing Teammates

To see all available teammates in your organization:

```bash
kubiya teammate list
```

This command displays a list of all teammates with their IDs, names, and descriptions.

For more detailed output in JSON format:

```bash
kubiya teammate list --output json
```

## Viewing Teammate Details

To get detailed information about a specific teammate:

```bash
kubiya teammate describe TEAMMATE_ID
```

This shows the teammate's configuration, capabilities, and associated resources.

## Creating a New Teammate

You can create teammates with specific capabilities tailored to your needs:

```bash
kubiya teammate create --name "Infrastructure-Expert" \
                      --description "Specialist in AWS and Kubernetes infrastructure" \
                      --capabilities "aws,kubernetes,terraform" \
                      --knowledge-items "item1,item2" \
                      --sources "source1,source2"
```

### Required Parameters:

- `--name`: A unique name for your teammate
- `--description`: A description of the teammate's purpose and expertise

### Optional Parameters:

- `--capabilities`: Comma-separated list of capabilities
- `--knowledge-items`: Knowledge items to associate with the teammate
- `--sources`: Sources containing tools the teammate can access
- `--avatar`: URL or path to an avatar image
- `--personality`: Personality traits for the teammate (e.g., "technical,concise")

## Updating a Teammate

To modify an existing teammate's configuration:

```bash
kubiya teammate update TEAMMATE_ID \
                      --description "Updated description" \
                      --capabilities "aws,kubernetes,terraform,docker" \
                      --knowledge-items "item1,item2,item3"
```

You can update any of the parameters you used when creating the teammate.

## Deleting a Teammate

To remove a teammate from your organization:

```bash
kubiya teammate delete TEAMMATE_ID
```

You'll be prompted to confirm the deletion.

## Chatting with Teammates

Once you've created teammates, you can interact with them in various ways:

### Interactive Chat

Start an interactive chat session with a specific teammate:

```bash
kubiya chat --teammate "Infrastructure-Expert" --interactive
```

This opens a chat session where you can have a conversation with the teammate.

### One-off Questions

Ask a specific question without starting an interactive session:

```bash
kubiya chat --teammate "Infrastructure-Expert" "How do I set up an AWS S3 bucket?"
```

### Providing Context

You can provide context to help the teammate understand your question better:

```bash
kubiya chat --teammate "Infrastructure-Expert" "What's wrong with this Terraform code?" --files "main.tf,variables.tf"
```

Or use stdin to pipe data:

```bash
cat error.log | kubiya chat --teammate "Infrastructure-Expert" "What's causing these errors?"
```

## Managing Chat Sessions

### Continuing Previous Sessions

To continue a previous conversation with a teammate:

```bash
# List active sessions
kubiya chat --list-sessions

# Continue a specific session
kubiya chat --interactive --session SESSION_ID
```

### Clearing Session Context

To start fresh while keeping the same session:

```bash
kubiya chat --interactive --clear-session
```

## Creating Specialized Teammates

Here are examples of creating teammates for specific roles:

### DevOps Teammate

```bash
kubiya teammate create --name "DevOps-Expert" \
                      --description "Specializes in CI/CD, infrastructure as code, and container orchestration" \
                      --capabilities "aws,kubernetes,terraform,jenkins,gitlab,docker" \
                      --sources "devops-tools,kubernetes-tools"
```

### Security Teammate

```bash
kubiya teammate create --name "Security-Expert" \
                      --description "Focused on security best practices, vulnerability assessment, and compliance" \
                      --capabilities "security,compliance,aws-security,kubernetes-security" \
                      --knowledge-items "security-best-practices,compliance-frameworks"
```

### Database Teammate

```bash
kubiya teammate create --name "Database-Expert" \
                      --description "Specializes in database management, optimization, and troubleshooting" \
                      --capabilities "mysql,postgresql,mongodb,redis,dynamodb" \
                      --sources "database-tools"
```

## Best Practices

1. **Create Specialized Teammates**: Rather than having one general-purpose teammate, create multiple specialized teammates for different domains.

2. **Provide Detailed Descriptions**: Include clear descriptions when creating teammates to help users understand their capabilities.

3. **Associate Relevant Sources**: Connect teammates with the appropriate tool sources to enhance their capabilities.

4. **Utilize Knowledge Items**: Add relevant knowledge items to teammates to improve their domain expertise.

5. **Start with Templates**: Use pre-built templates as a starting point for common teammate roles.

```bash
kubiya teammate create --from-template "devops"
```

## End-to-End Example: Creating and Using a Cloud Infrastructure Teammate

Here's a complete workflow for creating and interacting with a specialized cloud infrastructure teammate:

```bash
# Step 1: Create a new teammate
kubiya teammate create --name "Cloud-Expert" \
                      --description "Specialist in AWS, Azure, and GCP infrastructure" \
                      --capabilities "aws,azure,gcp,terraform,pulumi" \
                      --sources "cloud-tools"

# Step 2: Get the teammate ID
TEAMMATE_ID=$(kubiya teammate list --output json | jq -r '.[] | select(.name=="Cloud-Expert") | .id')

# Step 3: Add knowledge items to the teammate
kubiya knowledge create --title "AWS Best Practices" --content "aws-practices.md"
kubiya knowledge create --title "Multi-Cloud Strategy" --content "multi-cloud.md"
kubiya teammate update $TEAMMATE_ID --knowledge-items "AWS Best Practices,Multi-Cloud Strategy"

# Step 4: Start an interactive chat
kubiya chat --teammate "Cloud-Expert" --interactive

# Step 5: Ask specific questions with context
kubiya chat --teammate "Cloud-Expert" "How can I optimize this infrastructure?" --files "terraform/*.tf"
```

For more information about specific teammate capabilities and configuration options, see the [official documentation](https://docs.kubiya.ai).
EOL

echo "Teammate management documentation created successfully."
