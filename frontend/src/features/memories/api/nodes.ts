import { api as globalApi } from '../../../services';

export const nodesApi = {
    createNode: globalApi.createNode.bind(globalApi),
    listNodes: globalApi.listNodes.bind(globalApi),
    getNode: globalApi.getNode.bind(globalApi),
    updateNode: globalApi.updateNode.bind(globalApi),
    deleteNode: globalApi.deleteNode.bind(globalApi),
    bulkDeleteNodes: globalApi.bulkDeleteNodes.bind(globalApi),
    getNodeCategories: globalApi.getNodeCategories.bind(globalApi),
    categorizeNode: globalApi.categorizeNode.bind(globalApi),
    getGraphData: globalApi.getGraphData.bind(globalApi)
};
