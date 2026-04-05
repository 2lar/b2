import { api } from '../../../services';

export const searchApi = {
    search: api.searchNodes.bind(api),
};
