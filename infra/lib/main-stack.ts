/**
 * Main Stack - Orchestrates all Brain2 infrastructure stacks
 */

import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { EnvironmentConfig } from './config/environments';
import { DatabaseStack } from './stacks/database-stack';
import { ComputeStack } from './stacks/compute-stack';
import { ApiStack } from './stacks/api-stack';
import { FrontendStack } from './stacks/frontend-stack';
import { MonitoringStack } from './stacks/monitoring-stack';

export interface MainStackProps extends StackProps {
  config: EnvironmentConfig;
}

/**
 * Main orchestrator stack that coordinates all Brain2 infrastructure components
 */
export class MainStack extends Stack {
  public readonly databaseStack: DatabaseStack;
  public readonly computeStack: ComputeStack;
  public readonly apiStack: ApiStack;
  public readonly frontendStack: FrontendStack;
  public readonly monitoringStack?: MonitoringStack;

  constructor(scope: Construct, id: string, props: MainStackProps) {
    super(scope, id, props);

    const { config } = props;

    // 1. Database Stack - DynamoDB tables and indexes
    this.databaseStack = new DatabaseStack(this, 'Database', {
      config,
      stackName: `${config.stackName}-database`,
    });

    // 2. Compute Stack - Lambda functions and EventBridge
    this.computeStack = new ComputeStack(this, 'Compute', {
      config,
      stackName: `${config.stackName}-compute`,
      memoryTable: this.databaseStack.memoryTable,
      connectionsTable: this.databaseStack.connectionsTable,
    });

    // 3. API Stack - HTTP API only (WebSocket moved to Compute Stack)
    this.apiStack = new ApiStack(this, 'Api', {
      config,
      stackName: `${config.stackName}-api`,
      backendLambda: this.computeStack.backendLambda,
      authorizerLambda: this.computeStack.authorizerLambda,
    });

    // 4. Frontend Stack - S3 bucket and CloudFront distribution
    this.frontendStack = new FrontendStack(this, 'Frontend', {
      config,
      stackName: `${config.stackName}-frontend`,
    });

    // 5. Monitoring Stack - CloudWatch dashboards and alarms (optional)
    if (config.monitoring.enableDashboards || config.monitoring.enableAlarms) {
      this.monitoringStack = new MonitoringStack(this, 'Monitoring', {
        config,
        stackName: `${config.stackName}-monitoring`,
        memoryTable: this.databaseStack.memoryTable,
        connectionsTable: this.databaseStack.connectionsTable,
        lambdaFunctions: [
          this.computeStack.backendLambda,
          this.computeStack.connectNodeLambda,
          this.computeStack.cleanupLambda,
          this.computeStack.wsConnectLambda,
          this.computeStack.wsDisconnectLambda,
          this.computeStack.wsSendMessageLambda,
          this.computeStack.authorizerLambda,
        ],
        httpApi: this.apiStack.httpApi.api,
        webSocketApi: this.computeStack.webSocketApi.api,
      });
    }

    // Set up dependencies to ensure proper deployment order
    // Note: Some dependencies are implicit through resource references
    this.computeStack.addDependency(this.databaseStack);
    // this.apiStack.addDependency(this.computeStack); // Implicit through Lambda references
    // this.frontendStack.addDependency(this.apiStack); // Not needed for S3/CloudFront

    if (this.monitoringStack) {
      this.monitoringStack.addDependency(this.apiStack);
      this.monitoringStack.addDependency(this.computeStack);
      this.monitoringStack.addDependency(this.databaseStack);
    }
  }
}