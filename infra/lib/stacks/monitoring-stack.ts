/**
 * Monitoring Stack - CloudWatch dashboards and alarms for Brain2
 */

import { Stack, StackProps, Duration } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as apigwv2 from 'aws-cdk-lib/aws-apigatewayv2';
import * as sns from 'aws-cdk-lib/aws-sns';
import * as actions from 'aws-cdk-lib/aws-cloudwatch-actions';
import { EnvironmentConfig } from '../config/environments';
import { MONITORING_CONFIG, getResourceName } from '../config/constants';

export interface MonitoringStackProps extends StackProps {
  config: EnvironmentConfig;
  memoryTable: dynamodb.Table;
  connectionsTable: dynamodb.Table;
  lambdaFunctions: lambda.Function[];
  httpApi: apigwv2.HttpApi;
  webSocketApi: apigwv2.WebSocketApi;
}

export class MonitoringStack extends Stack {
  public readonly dashboard?: cloudwatch.Dashboard;
  public readonly alarmTopic?: sns.Topic;

  constructor(scope: Construct, id: string, props: MonitoringStackProps) {
    super(scope, id, props);

    const {
      config,
      memoryTable,
      connectionsTable,
      lambdaFunctions,
      httpApi,
      webSocketApi,
    } = props;

    // Only create monitoring resources if enabled
    if (!config.monitoring.enableDashboards && !config.monitoring.enableAlarms) {
      return;
    }

    // SNS topic for alarms (if alarms are enabled)
    if (config.monitoring.enableAlarms) {
      this.alarmTopic = new sns.Topic(this, 'AlarmTopic', {
        topicName: getResourceName('alerts', config.resourcePrefix),
        displayName: `Brain2 ${config.stackName} Alerts`,
      });
    }

    // Create CloudWatch Dashboard (if enabled)
    if (config.monitoring.enableDashboards) {
      this.dashboard = this.createDashboard(props);
    }

    // Create alarms (if enabled)
    if (config.monitoring.enableAlarms && this.alarmTopic) {
      this.createAlarms(props);
    }
  }

  private createDashboard(props: MonitoringStackProps): cloudwatch.Dashboard {
    const { config, memoryTable, connectionsTable, lambdaFunctions, httpApi, webSocketApi } = props;

    const dashboard = new cloudwatch.Dashboard(this, 'Dashboard', {
      dashboardName: getResourceName('dashboard', config.resourcePrefix),
      periodOverride: cloudwatch.PeriodOverride.AUTO,
    });

    // API Gateway metrics
    const apiMetricsWidget = new cloudwatch.GraphWidget({
      title: 'API Gateway Metrics',
      left: [
        httpApi.metric('Count', {
          statistic: 'Sum',
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
        httpApi.metric('4XXError', {
          statistic: 'Sum',
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
        httpApi.metric('5XXError', {
          statistic: 'Sum',
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
      ],
      right: [
        httpApi.metric('Latency', {
          statistic: 'Average',
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
      ],
    });

    // Lambda metrics
    const lambdaErrorsWidget = new cloudwatch.GraphWidget({
      title: 'Lambda Function Errors',
      left: lambdaFunctions.map(func => 
        func.metricErrors({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        })
      ),
    });

    const lambdaDurationWidget = new cloudwatch.GraphWidget({
      title: 'Lambda Function Duration',
      left: lambdaFunctions.map(func => 
        func.metricDuration({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        })
      ),
    });

    const lambdaInvocationsWidget = new cloudwatch.GraphWidget({
      title: 'Lambda Function Invocations',
      left: lambdaFunctions.map(func => 
        func.metricInvocations({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        })
      ),
    });

    // DynamoDB metrics
    const dynamodbReadWidget = new cloudwatch.GraphWidget({
      title: 'DynamoDB Read Capacity',
      left: [
        memoryTable.metricConsumedReadCapacityUnits({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
        connectionsTable.metricConsumedReadCapacityUnits({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
      ],
    });

    const dynamodbWriteWidget = new cloudwatch.GraphWidget({
      title: 'DynamoDB Write Capacity',
      left: [
        memoryTable.metricConsumedWriteCapacityUnits({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
        connectionsTable.metricConsumedWriteCapacityUnits({
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
        }),
      ],
    });

    // WebSocket metrics
    const webSocketMetricsWidget = new cloudwatch.GraphWidget({
      title: 'WebSocket API Metrics',
      left: [
        new cloudwatch.Metric({
          namespace: 'AWS/ApiGatewayV2',
          metricName: 'Count',
          dimensionsMap: {
            ApiId: webSocketApi.apiId,
          },
          period: Duration.minutes(MONITORING_CONFIG.DASHBOARD_PERIOD_MINUTES),
          statistic: 'Sum',
        }),
      ],
    });

    // Add widgets to dashboard
    dashboard.addWidgets(
      apiMetricsWidget,
      lambdaErrorsWidget,
      lambdaDurationWidget,
      lambdaInvocationsWidget,
      dynamodbReadWidget,
      dynamodbWriteWidget,
      webSocketMetricsWidget,
    );

    return dashboard;
  }

  private createAlarms(props: MonitoringStackProps): void {
    const { config, lambdaFunctions, httpApi } = props;

    // API Gateway error rate alarm
    const apiErrorAlarm = new cloudwatch.Alarm(this, 'ApiErrorAlarm', {
      alarmName: getResourceName('api-error-rate', config.resourcePrefix),
      alarmDescription: 'High error rate in API Gateway',
      metric: new cloudwatch.MathExpression({
        expression: '(m1 + m2) / m3 * 100',
        usingMetrics: {
          m1: httpApi.metric('4XXError', {
            statistic: 'Sum',
            period: Duration.minutes(5),
          }),
          m2: httpApi.metric('5XXError', {
            statistic: 'Sum',
            period: Duration.minutes(5),
          }),
          m3: httpApi.metric('Count', {
            statistic: 'Sum',
            period: Duration.minutes(5),
          }),
        },
      }),
      threshold: MONITORING_CONFIG.ALARM_THRESHOLD_ERROR_RATE * 100, // Convert to percentage
      evaluationPeriods: MONITORING_CONFIG.ALARM_EVALUATION_PERIODS,
      treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
    });

    if (this.alarmTopic) {
      apiErrorAlarm.addAlarmAction(new actions.SnsAction(this.alarmTopic));
    }

    // Lambda function error alarms
    lambdaFunctions.forEach((func, index) => {
      const errorAlarm = new cloudwatch.Alarm(this, `LambdaErrorAlarm${index}`, {
        alarmName: getResourceName(`lambda-error-${func.functionName}`, config.resourcePrefix),
        alarmDescription: `High error rate in Lambda function ${func.functionName}`,
        metric: func.metricErrors({
          period: Duration.minutes(5),
        }),
        threshold: 5, // 5 errors in 5 minutes
        evaluationPeriods: MONITORING_CONFIG.ALARM_EVALUATION_PERIODS,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });

      const durationAlarm = new cloudwatch.Alarm(this, `LambdaDurationAlarm${index}`, {
        alarmName: getResourceName(`lambda-duration-${func.functionName}`, config.resourcePrefix),
        alarmDescription: `High duration in Lambda function ${func.functionName}`,
        metric: func.metricDuration({
          period: Duration.minutes(5),
        }),
        threshold: MONITORING_CONFIG.ALARM_THRESHOLD_DURATION_MS,
        evaluationPeriods: MONITORING_CONFIG.ALARM_EVALUATION_PERIODS,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });

      if (this.alarmTopic) {
        errorAlarm.addAlarmAction(new actions.SnsAction(this.alarmTopic));
        durationAlarm.addAlarmAction(new actions.SnsAction(this.alarmTopic));
      }
    });
  }
}