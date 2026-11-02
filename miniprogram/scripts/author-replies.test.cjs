const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),ts=require('typescript'),path=require('node:path')
const source=fs.readFileSync(path.join(__dirname,'../miniprogram/pages/post/post.ts'),'utf8').split('Page({')[0].replace(/^import .*$/gm,'')+'\nglobalThis.fixture={toCommentItem,privateComment,displayComments}'
const context={formatDate:()=>'',assetUrl:()=>'',COMMENT_STATUS_APPROVED:2,COMMENT_STATUS_PENDING:1}
vm.runInNewContext(ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText,context)
const f=context.fixture
const root={id:'root',display_name:'reader',content:'question',status:2}
test('mini reply presentation accepts only approved single child linked to this root',()=>{
 assert.equal(f.toCommentItem({...root,author_reply:{id:'child',parent_id:'other',status:2}}).authorReply,undefined)
 assert.equal(f.toCommentItem({...root,author_reply:{id:'child',parent_id:'root',status:1}}).authorReply,undefined)
 const item=f.toCommentItem({...root,author_reply:{id:'child',parent_id:'root',status:2,content:'answer',author_reply:{content:'never recurse'}}})
 assert.equal(item.authorReply.content,'answer');assert.equal(item.authorReply.authorReply,undefined);assert.equal(item.authorReply.avatarUrl,'')
})
test('private reply folds under loaded root while retaining child recheck identity',()=>{
 const child=f.privateComment({comment:{id:'child',parent_id:'root',content:'answer',status:2},visible:false,can_recheck:true,safety_state:0})
 const items=[f.toCommentItem(root),child];const display=f.displayComments(items)
 assert.equal(display.length,1);assert.equal(display[0].id,'root');assert.equal(display[0].authorReply.id,'child');assert.equal(display[0].authorReply.canRecheck,true);assert.equal(display[0].authorReply.pending,true)
 assert.equal(items[1].id,'child');assert.equal(f.displayComments([child]).length,1)
})
