# AWS Resource Monitor - Full-Stack Application

A comprehensive full-stack application for monitoring and managing AWS resources with cost analysis, built with Next.js, TypeScript, and AWS SDK.

## 🚀 Features

### Frontend
- **Modern React/Next.js Application** with TypeScript
- **Responsive Dashboard** with real-time AWS resource monitoring
- **Interactive Charts** using Recharts for data visualization
- **User Authentication** with NextAuth.js and role-based access control
- **Tailwind CSS** for modern, responsive design
- **Resource Management Interface** for creating and deleting AWS resources

### Backend
- **RESTful API Routes** for AWS resource management
- **AWS SDK Integration** for EC2, S3, RDS, Lambda, and Cost Explorer
- **Authentication Middleware** with JWT tokens
- **Role-based Authorization** (Admin, User, Viewer)
- **Error Handling** and logging

### AWS Integration
- **EC2 Instance Management** - List, create, and terminate instances
- **S3 Bucket Management** - List, create, and delete buckets
- **RDS Database Monitoring** - View database instances and status
- **Lambda Function Monitoring** - List and monitor Lambda functions
- **Cost Explorer Integration** - Real-time cost analysis and billing data
- **CloudWatch Metrics** - Resource usage monitoring

## 📋 Prerequisites

- Node.js 18+ 
- AWS Account with proper IAM permissions
- Vercel account (for deployment)

## 🛠️ Installation

### 1. Clone the Repository
```bash
git clone <repository-url>
cd aws-resource-monitor
```

### 2. Install Dependencies
```bash
npm install
```

### 3. Environment Configuration
Copy the environment template:
```bash
cp .env.example .env.local
```

Update `.env.local` with your configuration:
```env
# AWS Configuration
AWS_ACCESS_KEY_ID=your_aws_access_key_id
AWS_SECRET_ACCESS_KEY=your_aws_secret_access_key
AWS_REGION=us-east-1

# NextAuth Configuration
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=your_nextauth_secret_key_here
```

### 4. AWS IAM Permissions

Create an IAM user with the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances",
                "ec2:RunInstances",
                "ec2:TerminateInstances",
                "ec2:DescribeImages",
                "ec2:DescribeSecurityGroups",
                "s3:ListAllMyBuckets",
                "s3:CreateBucket",
                "s3:DeleteBucket",
                "s3:GetBucketLocation",
                "s3:ListBucket",
                "rds:DescribeDBInstances",
                "lambda:ListFunctions",
                "lambda:GetFunction",
                "cloudwatch:GetMetricStatistics",
                "cloudwatch:ListMetrics",
                "ce:GetCostAndUsage",
                "ce:GetDimensionValues"
            ],
            "Resource": "*"
        }
    ]
}
```

## 🚀 Development

### Start Development Server
```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

### Build for Production
```bash
npm run build
```

### Type Checking
```bash
npm run type-check
```

## 📱 Usage

### Authentication
The application includes demo accounts:
- **Admin**: `admin@example.com` / `admin123`
- **User**: `user@example.com` / `user123`

### Dashboard Features
1. **Resource Overview** - View counts of all AWS resources
2. **Cost Analysis** - Monitor spending across services
3. **Resource Management** - Create and delete resources
4. **Real-time Monitoring** - Live updates of resource status

### API Endpoints

#### Authentication
- `POST /api/auth/signin` - User authentication
- `POST /api/auth/signout` - User logout

#### AWS Resources
- `GET /api/aws/ec2` - List EC2 instances
- `POST /api/aws/ec2` - Create EC2 instance
- `DELETE /api/aws/ec2` - Terminate EC2 instance
- `GET /api/aws/s3` - List S3 buckets
- `POST /api/aws/s3` - Create S3 bucket
- `DELETE /api/aws/s3` - Delete S3 bucket
- `GET /api/aws/rds` - List RDS instances
- `GET /api/aws/lambda` - List Lambda functions
- `GET /api/aws/costs` - Get cost and usage data

## 🚀 Vercel Deployment

### 1. Install Vercel CLI
```bash
npm install -g vercel
```

### 2. Deploy to Vercel
```bash
vercel
```

### 3. Set Environment Variables
In the Vercel dashboard, add the following environment variables:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`
- `NEXTAUTH_SECRET`
- `NEXTAUTH_URL`

### 4. Configure Domain
Update `NEXTAUTH_URL` to your Vercel domain.

## 🏗️ Project Structure

```
aws-resource-monitor/
├── src/
│   ├── app/                    # Next.js 13+ app directory
│   │   ├── api/               # API routes
│   │   │   ├── auth/         # Authentication routes
│   │   │   └── aws/          # AWS service routes
│   │   ├── auth/             # Authentication pages
│   │   ├── globals.css       # Global styles
│   │   ├── layout.tsx        # Root layout
│   │   ├── page.tsx          # Dashboard page
│   │   └── providers.tsx     # Context providers
│   ├── components/            # React components
│   │   ├── dashboard/        # Dashboard components
│   │   └── ui/              # UI components
│   ├── lib/                  # Utility libraries
│   │   ├── aws-config.ts     # AWS SDK configuration
│   │   ├── aws-services.ts   # AWS service functions
│   │   └── auth.ts          # Authentication logic
│   ├── types/                # TypeScript type definitions
│   │   ├── auth.ts          # Auth types
│   │   └── aws.ts           # AWS types
│   └── utils/                # Utility functions
├── public/                   # Static assets
├── .env.example             # Environment template
├── .gitignore              # Git ignore rules
├── next.config.js          # Next.js configuration
├── package.json            # Dependencies
├── tailwind.config.js      # Tailwind CSS configuration
├── tsconfig.json           # TypeScript configuration
└── vercel.json             # Vercel deployment configuration
```

## 🔧 Configuration

### AWS Configuration
- Configure AWS credentials and region in environment variables
- Ensure proper IAM permissions for all required services
- Test AWS connectivity before deployment

### Authentication
- Uses NextAuth.js with credentials provider
- Supports role-based access control
- JWT tokens for session management

### Styling
- Tailwind CSS for responsive design
- Custom component library
- Dark/light mode support (configurable)

## 🔒 Security

- Environment variables for sensitive data
- Role-based access control
- API route protection with authentication middleware
- Input validation and sanitization
- AWS IAM least privilege principle

## 🐛 Troubleshooting

### Common Issues

1. **AWS Credentials Error**
   - Verify AWS credentials in environment variables
   - Check IAM permissions
   - Ensure AWS region is correct

2. **Authentication Issues**
   - Verify NEXTAUTH_SECRET is set
   - Check NEXTAUTH_URL matches your domain
   - Clear browser cookies and try again

3. **API Errors**
   - Check browser console for detailed error messages
   - Verify AWS service availability in your region
   - Check API route permissions

## 📝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License.

## 🆘 Support

For support and questions:
- Create an issue on GitHub
- Check the troubleshooting section
- Review AWS documentation for service-specific issues

## 🚀 Future Enhancements

- [ ] Advanced cost optimization recommendations
- [ ] Resource tagging and organization
- [ ] CloudFormation template management
- [ ] Multi-region support
- [ ] Advanced monitoring and alerting
- [ ] Database integration for historical data
- [ ] Mobile app support
- [ ] Advanced user management
