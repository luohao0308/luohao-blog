"""Offline checks for the blog command and its two source archives."""
import io
from pathlib import Path
import sys
import tempfile
from types import ModuleType, SimpleNamespace
import unittest
from unittest.mock import Mock, patch
import zipfile

DEPLOY = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(DEPLOY))
import retained_frontend_release as cli


def framework_module(name, **attributes):
    module = ModuleType(name)
    vars(module).update(attributes)
    return module


def archive_files(raw):
    with zipfile.ZipFile(io.BytesIO(raw)) as package:
        return {name: package.read(name) for name in package.namelist()}


class ReleaseCommandTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.workspace = Path(temp.name).resolve()
        self.checkout = self.workspace / 'framework'
        self.checkout.mkdir()
        self.runtime = '1' * 40
        self.blog = '2' * 40
        self.args = ['--runtime-sha', self.runtime, '--runtime-checkout', str(self.checkout),
                     '--workspace', str(self.workspace)]

    def test_blog_pin_checks_command_and_probe_bytes(self):
        source = (DEPLOY.parent / cli._PROBE).read_bytes()
        with (patch.object(cli, '_published_sha', return_value=self.blog) as main,
              patch.object(cli, '_git', side_effect=[Path(cli.__file__).read_bytes(), source]) as git):
            self.assertEqual(cli._published_blog(DEPLOY.parent), (self.blog, source))
            main.assert_called_once_with(cli._BLOG_REPOSITORY)
            self.assertEqual(git.call_args.args, (DEPLOY.parent, 'show', f'{self.blog}:{cli._PROBE}'))
        with (patch.object(cli, '_published_sha', return_value=self.blog),
              patch.object(cli, '_git', side_effect=[Path(cli.__file__).read_bytes(), b'other']),
              self.assertRaisesRegex(cli.ReleaseCommandBlocked, 'probe differs')):
            cli._published_blog(DEPLOY.parent)

    def test_unpublished_runtime_cannot_load_or_dispatch(self):
        with (patch.object(cli, '_published_sha', return_value='3' * 40),
              patch.object(cli, '_runtime') as runtime, patch.object(cli, '_publish') as publish,
              patch.object(cli, '_emit')):
            self.assertEqual(cli.main(['preflight', *self.args]), 2)
        runtime.assert_not_called()
        publish.assert_not_called()

    def test_runtime_without_release_client_blocks_before_production_inspection(self):
        modules = {
            'deployment_compose_production_frontend_review_v1_host': framework_module(
                'review', GitHubProductionFrontendReviewV1=object),
            'deployment_compose_retained_source_v1_host': framework_module(
                'source', production_archive=Mock()),
            'deployment_compose_witness_store': framework_module('store', canonical=Mock()),
        }
        with (patch.dict(sys.modules, modules), patch.object(cli, '_published_sha', return_value=self.runtime),
              patch.object(cli, '_published_blog', return_value=(self.blog, b'probe')),
              patch.object(cli, '_runtime'), patch.object(cli, '_prepare') as prepare,
              patch.object(cli, '_emit') as emit):
            self.assertEqual(cli.main(['preflight', *self.args]), 2)
        prepare.assert_not_called()
        self.assertIn('lacks the retained release approval client', emit.call_args.kwargs['reason'])

    def test_preflight_is_read_only_and_drift_blocks_release(self):
        reviewer = SimpleNamespace(request_release_review=Mock(), release_stage_grant=Mock())
        production_archive = Mock(return_value=b'framework')
        modules = {
            'deployment_compose_retained_source_v1_host': framework_module(
                'source', production_archive=production_archive),
            'deployment_compose_production_frontend_review_v1_host': framework_module(
                'review', GitHubProductionFrontendReviewV1=reviewer),
            'deployment_compose_witness_store': framework_module('store', canonical=lambda value: repr(value).encode()),
        }
        probe = Mock()
        probe.inspection_archive.return_value = b'inspection'
        before = dict(runtimeSha=self.runtime, blogSourceSha=self.blog, journalDigest='4' * 64)
        after = dict(before, journalDigest='5' * 64)
        release_args = ['release', *self.args, '--recipient-private-key', str(self.workspace / 'key')]
        with (patch.dict(sys.modules, modules), patch.object(cli, '_published_sha', return_value=self.runtime),
              patch.object(cli, '_published_blog', return_value=(self.blog, b'probe')),
              patch.object(cli, '_runtime'), patch.object(cli, '_load_probe', return_value=probe),
              patch.object(cli, '_prepare', side_effect=[({}, {}, before), ({}, {}, after), ({}, {}, before)]) as prep,
              patch.object(cli, '_publish', return_value=0) as publish, patch.object(cli, '_emit')):
            self.assertEqual(cli.main(['preflight', *self.args]), 0)
            publish.assert_not_called()
            prep.assert_called_with(probe, b'inspection', self.runtime, self.blog)
            production_archive.assert_called_with(self.checkout, self.runtime)
            probe.inspection_archive.assert_called_with(b'framework', b'probe')
            self.assertEqual(cli.main(release_args), 2)
            publish.assert_not_called()
            self.assertEqual(cli.main(release_args), 0)
            self.assertEqual(publish.call_args.args[1:4], (b'framework', probe, b'inspection'))
        self.assertEqual((self.workspace / 'baseline.json').stat().st_mode & 0o777, 0o600)

    def execute_stages(self, switch, restored=None):
        controller, reviewer, probe = Mock(), Mock(), Mock()
        controller.review = {'retainedCutover': {'creationRequestHash': '1' * 64, 'scopeGeneration': 7}}
        results = [dict(blocked=False, switchReview=controller.review, result={'requestHash': '1' * 64}), switch]
        if restored is not None:
            results.append(restored)
        controller.execute.side_effect = results
        reviewer.release_stage_grant.side_effect = [
            (index, SimpleNamespace(request=object()), object()) for index in range(1, len(results) + 1)
        ]
        pending = dict(requestHash='1' * 64, generation=7)
        probe.inspect_active_release.return_value = pending
        modules = {
            'deployment_compose_witness_store': framework_module('store', canonical=lambda value: repr(value).encode()),
            'deployment_gate_witness': framework_module('gate', key_material=lambda value: value),
        }
        with patch.dict(sys.modules, modules):
            try:
                result = cli._execute_release(controller, reviewer, probe, b'inspection', {}, {},
                                              self.workspace, b'witness key', Mock())
            except cli.ReleaseCommandBlocked as error:
                result = error
        reviewer.request_release_review.assert_called_once()
        probe.inspect_active_release.assert_called_with(b'inspection', {}, request_hash='1' * 64, generation=7)
        return result, [call.args[0] for call in reviewer.release_stage_grant.call_args_list]

    def test_healthy_candidate_uses_one_review_and_no_rollback(self):
        result, stages = self.execute_stages(dict(terminalProof='a' * 64, result={'candidateId': 'b' * 64}))
        self.assertEqual(result['disposition'], 'retained-candidate')
        self.assertEqual(result['proof'], 'a' * 64)
        self.assertEqual(stages, ['create', 'switch'])

    def test_unverified_failure_and_readiness_failure_use_different_paths(self):
        error, stages = self.execute_stages(dict(restoreReview=None))
        self.assertIsInstance(error, cli.ReleaseCommandBlocked)
        self.assertEqual(stages, ['create', 'switch'])
        result, stages = self.execute_stages(dict(restoreReview={}, readinessFailure=object()),
                                             dict(blocked=False, result={'originalId': 'c' * 64}))
        self.assertEqual(result['disposition'], 'restored-original')
        self.assertEqual(stages, ['create', 'switch', 'rollback'])


class ProbeArchiveTests(unittest.TestCase):
    def test_probe_only_appears_in_inspection_archive(self):
        modules = {
            'deployment_compose_host_snapshot_v3_host': dict(HostSnapshotV3=object),
            'deployment_compose_production_caddy_probe_v1_host': dict(ProductionCaddyProbeV1=object),
            'deployment_compose_production_scope_v1_host': dict.fromkeys(('_directory', '_history', '_name', '_read')),
            'deployment_compose_retained_native_entry_v1_host': dict.fromkeys(('_inspect_registration', 'scope_registration')),
            'deployment_compose_retained_source_v1_host': dict(
                MAX_ARCHIVE=10000, MAX_SOURCE=20000, NAME=r'.+\.py', archive_files=archive_files),
            'deployment_compose_witness_store': dict.fromkeys(('canonical', 'hex_value', 'pairs')),
        }
        source = (DEPLOY.parent / cli._PROBE).read_bytes()
        with patch.dict(sys.modules, {name: framework_module(name, **attrs) for name, attrs in modules.items()}):
            probe = cli._load_probe(source)
        output = io.BytesIO()
        with zipfile.ZipFile(output, 'w') as archive:
            archive.writestr('scripts/framework.py', b'framework code')
        runtime = output.getvalue()
        inspection = probe.inspection_archive(runtime, source)
        self.assertEqual(archive_files(runtime), {'scripts/framework.py': b'framework code'})
        self.assertEqual(archive_files(inspection), {**archive_files(runtime), probe._MEMBER: source})
        with self.assertRaises(ValueError):
            probe.inspection_archive(inspection, source)
