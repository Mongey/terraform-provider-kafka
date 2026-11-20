import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from terraform_benchmark import variant_environment, broker_requests, provider_trace, validate_plan, network_delay


class HarnessTests(unittest.TestCase):
    def test_sdk_level_applies_only_to_candidates(self):
        env = {"TF_IN_AUTOMATION": "1"}
        self.assertEqual(variant_environment(env, "baseline", "WARN"), env)
        self.assertEqual(variant_environment(env, "candidate", "WARN"),
                         {**env, "TF_LOG_SDK": "WARN"})
        self.assertEqual(variant_environment(env, "candidate2", None), env)
        self.assertNotIn("TF_LOG_SDK", env)

    def test_network_delay_scope_and_readback(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'network.log'
            def response(command, **kwargs):
                output.write_text('qdisc netem 30: delay 1s\n Sent 100 bytes 2 pkt (dropped 0)')
            with patch('terraform_benchmark.run', side_effect=response) as execute:
                network_delay('fixture-broker', 1000, {}, output, configure=True)
                command = execute.call_args.args[0]
                self.assertIn('--network=container:fixture-broker', command)
                self.assertIn('match ip sport 9092 0xffff flowid 1:3', command[-1])
                network_delay('fixture-broker', 1000, {}, output)
                self.assertNotIn('qdisc add', execute.call_args.args[0][-1])
            with patch('terraform_benchmark.run'):
                output.write_text('qdisc netem 30: delay 1s\n Sent 0 bytes 0 pkt (dropped 0)')
                with self.assertRaisesRegex(ValueError, 'no packets'):
                    network_delay('fixture-broker', 1000, {}, output)

    def test_wire_log_filters_helper_and_preserves_resources(self):
        request = {'requestHeader': {'clientId': 'terraform-provider-kafka', 'requestApiKey': 32},
                   'request': {'resources': [{'resourceName': 'refresh-topic-0000'}]}}
        helper = copy.deepcopy(request)
        helper['requestHeader']['clientId'] = 'terraform-refresh-harness'
        lines = '\n'.join('DEBUG Completed request:' + json.dumps(r) + ' (kafka.request.logger)'
                          for r in (helper, request))
        self.assertEqual(broker_requests(lines), [request])
        with self.assertRaises(json.JSONDecodeError):
            broker_requests('Completed request:invalid')

    def test_trace_counts_only_refresh_reads(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'trace'
            path.write_text("2026-11-18T12:00:00.000Z provider: 2026/11/18 12:00:00 [DEBUG] reading topic 'a' from Kafka: false\n"
                            "2026-11-18T12:00:00.100Z provider: 2026/11/18 12:00:00 [TRACE] Read 2 config entries from Kafka\n"
                            "2026-11-18T12:00:00.200Z framework log @module=sdk.proto\n")
            trace = provider_trace(path)
            self.assertEqual(trace['topic_reads'], 1)
            self.assertEqual(trace['config_completions'], 1)
            self.assertEqual(trace['provider_log_lines'], 2)
            self.assertEqual(trace['sdk_log_lines'], 1)
            self.assertEqual(trace['trace_bytes'], path.stat().st_size)
            self.assertAlmostEqual(trace['topic_read_span_s'], .1, places=5)

    def test_exact_plan_and_alias_isolation(self):
        resources = []
        for alias, base in (("first", 60000), ("second", 120000)):
            for index in range(49):
                name = f"refresh-topic-{index:04d}"
                for kind in ("kafka_topic", "kafka_acl"):
                    values = ({'name': name, 'partitions': 1, 'replication_factor': 3,
                               'config': {'retention.ms': str(base + index),
                                          'cleanup.policy': 'compact' if index % 2 == 0 else 'delete'}}
                              if kind == 'kafka_topic' else
                              {'resource_name': name, 'acl_principal': f'User:refresh-bench-{index}',
                               'acl_operation': 'Read', 'acl_permission_type': 'Allow'})
                    resources.append({'address': f'{kind}.{alias}["{name}"]', 'type': kind,
                                      'name': alias, 'index': name, 'values': values})
        document = {'prior_state': {'values': {'root_module': {'resources': resources}}},
                    'resource_changes': [{'change': {'actions': ['no-op']}} for _ in resources]}
        fingerprint = validate_plan(document, 49)
        resources.reverse()
        self.assertEqual(validate_plan(document, 49), fingerprint)
        broken = copy.deepcopy(document)
        next(r for r in broken['prior_state']['values']['root_module']['resources']
             if r['type'] == 'kafka_topic')['values']['config']['retention.ms'] = 'wrong'
        with self.assertRaisesRegex(ValueError, 'config'):
            validate_plan(broken, 49)
        broken = copy.deepcopy(document)
        broken['resource_changes'][0]['change']['actions'] = ['update']
        with self.assertRaisesRegex(ValueError, 'unexpected changes'):
            validate_plan(broken, 49)

    def test_plan_rejects_missing_resources(self):
        with self.assertRaisesRegex(ValueError, 'resource count'):
            validate_plan({'prior_state': {'values': {'root_module': {'resources': []}}}}, 593)


if __name__ == '__main__':
    unittest.main()
