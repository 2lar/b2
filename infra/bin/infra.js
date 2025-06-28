#!/usr/bin/env node
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
require("dotenv/config"); // Loads variables from .env into process.env
require("source-map-support/register");
const cdk = require("aws-cdk-lib");
const b2_stack_1 = require("../lib/b2-stack");
const app = new cdk.App();
new b2_stack_1.b2Stack(app, 'b2Stack', {
    env: {
        account: process.env.CDK_DEFAULT_ACCOUNT,
        region: process.env.CDK_DEFAULT_REGION,
    },
    description: 'Brain2 - Graph-based knowledge management system',
});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoiaW5mcmEuanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJpbmZyYS50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7QUFDQSx5QkFBdUIsQ0FBQyw2Q0FBNkM7QUFFckUsdUNBQXFDO0FBQ3JDLG1DQUFtQztBQUNuQyw4Q0FBMEM7QUFFMUMsTUFBTSxHQUFHLEdBQUcsSUFBSSxHQUFHLENBQUMsR0FBRyxFQUFFLENBQUM7QUFFMUIsSUFBSSxrQkFBTyxDQUFDLEdBQUcsRUFBRSxTQUFTLEVBQUU7SUFDMUIsR0FBRyxFQUFFO1FBQ0gsT0FBTyxFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsbUJBQW1CO1FBQ3hDLE1BQU0sRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLGtCQUFrQjtLQUN2QztJQUNELFdBQVcsRUFBRSxrREFBa0Q7Q0FDaEUsQ0FBQyxDQUFDIiwic291cmNlc0NvbnRlbnQiOlsiIyEvdXNyL2Jpbi9lbnYgbm9kZVxuaW1wb3J0ICdkb3RlbnYvY29uZmlnJzsgLy8gTG9hZHMgdmFyaWFibGVzIGZyb20gLmVudiBpbnRvIHByb2Nlc3MuZW52XG5cbmltcG9ydCAnc291cmNlLW1hcC1zdXBwb3J0L3JlZ2lzdGVyJztcbmltcG9ydCAqIGFzIGNkayBmcm9tICdhd3MtY2RrLWxpYic7XG5pbXBvcnQgeyBiMlN0YWNrIH0gZnJvbSAnLi4vbGliL2IyLXN0YWNrJztcblxuY29uc3QgYXBwID0gbmV3IGNkay5BcHAoKTtcblxubmV3IGIyU3RhY2soYXBwLCAnYjJTdGFjaycsIHtcbiAgZW52OiB7XG4gICAgYWNjb3VudDogcHJvY2Vzcy5lbnYuQ0RLX0RFRkFVTFRfQUNDT1VOVCxcbiAgICByZWdpb246IHByb2Nlc3MuZW52LkNES19ERUZBVUxUX1JFR0lPTixcbiAgfSxcbiAgZGVzY3JpcHRpb246ICdCcmFpbjIgLSBHcmFwaC1iYXNlZCBrbm93bGVkZ2UgbWFuYWdlbWVudCBzeXN0ZW0nLFxufSk7Il19