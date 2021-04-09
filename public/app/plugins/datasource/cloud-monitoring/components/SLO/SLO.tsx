import React, { useEffect, useState } from 'react';
import { InlineFields, Select } from '@grafana/ui';
import { SelectableValue } from '@grafana/data';
import CloudMonitoringDatasource from '../../datasource';
import { SLOQuery } from '../../types';
import { LABEL_WIDTH, SELECT_WIDTH } from '../../constants';

export interface Props {
  onChange: (query: SLOQuery) => void;
  query: SLOQuery;
  templateVariableOptions: Array<SelectableValue<string>>;
  datasource: CloudMonitoringDatasource;
}

export const SLO: React.FC<Props> = ({ query, templateVariableOptions, onChange, datasource }) => {
  const [slos, setSLOs] = useState<Array<SelectableValue<string>>>([]);

  useEffect(() => {
    if (!query.projectName || !query.serviceId) {
      return;
    }

    datasource
      .getServiceLevelObjectives(query.projectName, query.serviceId)
      .then((sloIds: Array<SelectableValue<string>>) => {
        setSLOs([
          {
            label: 'Template Variables',
            options: templateVariableOptions,
          },
          ...sloIds,
        ]);
      });
  }, [datasource, query, templateVariableOptions]);

  return (
    <InlineFields label="SLO" grow transparent labelWidth={LABEL_WIDTH}>
      <Select
        width={SELECT_WIDTH}
        allowCustomValue
        value={query?.sloId && { value: query?.sloId, label: query?.sloName || query?.sloId }}
        placeholder="Select SLO"
        options={slos}
        onChange={async ({ value: sloId = '', label: sloName = '' }) => {
          const slos = await datasource.getServiceLevelObjectives(query.projectName, query.serviceId);
          const slo = slos.find(({ value }) => value === datasource.templateSrv.replace(sloId));
          onChange({ ...query, sloId, sloName, goal: slo?.goal });
        }}
      />
    </InlineFields>
  );
};
